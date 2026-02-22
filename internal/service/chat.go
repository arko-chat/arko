package service

import (
	"bytes"
	"context"
	"fmt"
	"sync"

	"github.com/arko-chat/arko/components/ui"
	"github.com/arko-chat/arko/components/utils"
	"github.com/arko-chat/arko/internal/matrix"
	"github.com/arko-chat/arko/internal/models"
	"github.com/arko-chat/arko/internal/ws"
	"github.com/puzpuzpuz/xsync/v4"
)

type chatListener struct {
	id     uint64
	cancel context.CancelFunc
}

type ChatService struct {
	*BaseService
	messages   *xsync.Map[string, *models.MessageTree]
	listeners  *xsync.Map[string, *chatListener]
	listenerMu sync.Mutex
}

func NewChatService(
	mgr *matrix.Manager,
	hub *ws.Hub,
) *ChatService {
	return &ChatService{
		BaseService: NewBaseService(mgr, hub),
		messages:    xsync.NewMap[string, *models.MessageTree](),
		listeners:   xsync.NewMap[string, *chatListener](),
	}
}

func (s *ChatService) GetRoomMessages(
	ctx context.Context,
	roomID string,
) (*models.MessageTree, error) {
	userID := s.GetCurrentUserID()

	messageTree, _ := s.messages.LoadOrCompute(roomID, func() (newValue *models.MessageTree, cancel bool) {
		messages, _ := s.matrix.GetRoomMessages(ctx, userID, roomID, "", "", 50)
		for _, message := range messages {
			newValue.BTreeG.Set(message)
		}

		if _, ok := s.listeners.Load(roomID); !ok {
			mxSession := s.matrix.GetMatrixSession(userID)
			if mxSession != nil {
				listenerCtx, cancel := context.WithCancel(context.Background())
				ch, id := mxSession.ListenMessages()
				s.listeners.Store(roomID, &chatListener{
					id:     id,
					cancel: cancel,
				})
				go func() {
					for {
						select {
						case <-listenerCtx.Done():
							return
						case evt, ok := <-ch:
							if !ok {
								return
							}
							msg := s.matrix.EventToMessage(
								s.matrix.GetCurrentMatrixSession().GetClient(), evt,
							)
							if msg == nil || msg.ChannelID != roomID {
								continue
							}
							newValue.Set(*msg)
						}
					}
				}()
			} else {
				return newValue, true
			}
		}

		return newValue, false
	})

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
	return s.matrix.SendMessage(ctx, userID, roomID, content, nonce)
}

func (s *ChatService) InsertAndBroadcast(
	ctx context.Context,
	channelID string,
	msg models.Message,
) error {
	tree, _ := s.messages.LoadOrStore(channelID, models.NewMessageTree())
	_, replaced := tree.Set(msg)
	if replaced {
		return nil
	}

	neighbors := tree.GetNeighbors(msg)
	continued := neighbors.Prev != nil &&
		neighbors.Prev.Author.ID == msg.Author.ID &&
		utils.WithinMinutes(neighbors.Prev.Timestamp, msg.Timestamp, 5)

	var buf bytes.Buffer

	if neighbors.Next != nil {
		oobTarget := "beforebegin:#msg-" + neighbors.Next.ID
		err := renderInsertOOB(ctx, &buf, msg, continued, oobTarget)
		if err != nil {
			return err
		}
	} else if neighbors.Prev != nil {
		oobTarget := "afterend:#msg-" + neighbors.Prev.ID
		err := renderInsertOOB(ctx, &buf, msg, continued, oobTarget)
		if err != nil {
			return err
		}
	} else {
		oobTarget := "beforeend:#message-list"
		err := renderInsertOOB(ctx, &buf, msg, continued, oobTarget)
		if err != nil {
			return err
		}
	}

	if prev, isContinued := s.checkRegrouping(msg, neighbors); prev != nil {
		err := ui.MessageBubbleOOB(*prev, isContinued, "outerHTML").Render(ctx, &buf)
		if err != nil {
			return fmt.Errorf("render message oob: %w", err)
		}
	}

	s.hub.Broadcast(channelID, buf.Bytes())
	return nil
}

func renderInsertOOB(
	ctx context.Context,
	buf *bytes.Buffer,
	msg models.Message,
	continued bool,
	oobTarget string,
) error {
	var inner bytes.Buffer
	if continued {
		if err := ui.MessageBubbleContinued(msg).Render(ctx, &inner); err != nil {
			return fmt.Errorf("render message oob: %w", err)
		}
	} else {
		if err := ui.MessageBubble(msg).Render(ctx, &inner); err != nil {
			return fmt.Errorf("render message oob: %w", err)
		}
	}
	_, err := fmt.Fprintf(
		buf,
		`<div hx-swap-oob="%s">%s</div>`,
		oobTarget,
		inner.String(),
	)
	return err
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
