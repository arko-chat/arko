package service

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/arko-chat/arko/components/ui"
	"github.com/arko-chat/arko/components/utils"
	"github.com/arko-chat/arko/internal/matrix"
	"github.com/arko-chat/arko/internal/models"
	"github.com/arko-chat/arko/internal/ws"
	"github.com/puzpuzpuz/xsync/v4"
)

type ChatService struct {
	*BaseService
	messages *xsync.Map[string, *models.MessageTree]
}

func NewChatService(
	mgr *matrix.Manager,
	hub *ws.Hub,
) *ChatService {
	return &ChatService{
		BaseService: NewBaseService(mgr, hub),
	}
}

func (s *ChatService) GetRoomMessages(
	ctx context.Context,
	roomID string,
) (*models.MessageTree, error) {
	userID := s.GetCurrentUserID()

	messageTree, _ := s.messages.LoadOrStore(roomID, models.NewMessageTree())
	if messageTree.Len() == 0 {
		messages, _ := s.matrix.GetRoomMessages(ctx, userID, roomID, "", "", 50)
		for _, message := range messages {
			messageTree.Set(message)
		}
	}

	return messageTree, nil
}

func (s *ChatService) SendRoomMessage(
	ctx context.Context,
	roomID string,
	author models.User,
	content string,
	nonce string,
) error {
	userID := s.GetCurrentUserID()
	err := s.matrix.SendMessage(ctx, userID, roomID, content)
	if err != nil {
		return err
	}

	msg := models.Message{
		ID:        "",
		Content:   content,
		Author:    author,
		Timestamp: time.Now(),
		Nonce:     nonce,
	}

	return s.InsertAndBroadcast(ctx, roomID, msg)
}

func (s *ChatService) InsertAndBroadcast(
	ctx context.Context,
	channelID string,
	msg models.Message,
) error {
	tree, _ := s.messages.LoadOrStore(channelID, models.NewMessageTree())
	tree.Set(msg)

	neighbors := tree.GetNeighbors(msg)
	swap, continued := s.resolveSwap(msg, neighbors)

	var buf bytes.Buffer
	var rerenderBuf bytes.Buffer

	err := ui.MessageBubbleOOB(msg, continued, swap).Render(ctx, &buf)
	if err != nil {
		return fmt.Errorf("render message oob: %w", err)
	}

	if prev, continued := s.checkRegrouping(msg, neighbors); prev != nil {
		swap := fmt.Sprintf("outerHTML:#msg-%s", msg.ID)
		err := ui.MessageBubbleOOB(*prev, continued, swap).Render(ctx, &buf)
		if err != nil {
			return fmt.Errorf("render message oob: %w", err)
		}
		buf.Write(rerenderBuf.Bytes())
	}

	s.hub.Broadcast(channelID, buf.Bytes())
	return nil
}

func (s *ChatService) resolveSwap(
	msg models.Message,
	n models.Neighbors,
) (strategy string, continued bool) {
	if n.Next != nil {
		continued = n.Next.Author.ID == msg.Author.ID &&
			utils.WithinMinutes(msg.Timestamp, n.Next.Timestamp, 5)
		return "afterend:#msg-" + n.Next.ID, continued
	}

	if n.Prev != nil {
		return "beforebegin:#msg-" + n.Prev.ID, false
	}

	return "afterbegin:#message-list", false
}

func (s *ChatService) checkRegrouping(
	msg models.Message,
	n models.Neighbors,
) (*models.Message, bool) {
	if n.Prev == nil {
		return nil, false
	}

	prev := n.Prev
	wasContinued := false
	if n.Next != nil {
		wasContinued = n.Next.Author.ID == prev.Author.ID &&
			utils.WithinMinutes(prev.Timestamp, n.Next.Timestamp, 5)
	}

	isContinued := msg.Author.ID == prev.Author.ID &&
		utils.WithinMinutes(prev.Timestamp, msg.Timestamp, 5)

	if wasContinued != isContinued {
		return prev, isContinued
	}

	return nil, false
}
