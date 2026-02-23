package service

import (
	"bytes"
	"context"
	"fmt"

	"github.com/arko-chat/arko/components/ui"
	"github.com/arko-chat/arko/components/utils"
	"github.com/arko-chat/arko/internal/matrix"
	"github.com/arko-chat/arko/internal/models"
	"github.com/arko-chat/arko/internal/ws"
)

type chatListener struct {
	id     uint64
	cancel context.CancelFunc
}

type ChatService struct {
	*BaseService
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
) (*matrix.MessageTree, error) {
	userID := s.GetCurrentUserID()
	matrixSession := s.matrix.GetMatrixSession(userID)
	messageTree := matrixSession.GetMessageTree(roomID)

	// only initialized once internally
	messageTree.Listen(ctx, func(mte matrix.MessageTreeEvent) {
		neighbors := mte.Neighbors
		msg := mte.Message

		continued := neighbors.Prev != nil &&
			neighbors.Prev.Author.ID == msg.Author.ID &&
			utils.WithinMinutes(neighbors.Prev.Timestamp, msg.Timestamp, 5)

		var buf bytes.Buffer

		if neighbors.Next != nil {
			oobTarget := "beforebegin:#msg-" + neighbors.Next.ID
			err := renderInsertOOB(ctx, &buf, msg, continued, oobTarget)
			if err != nil {
				return
			}
		} else if neighbors.Prev != nil {
			oobTarget := "afterend:#msg-" + neighbors.Prev.ID
			err := renderInsertOOB(ctx, &buf, msg, continued, oobTarget)
			if err != nil {
				return
			}
		} else {
			oobTarget := "beforeend:#message-list"
			err := renderInsertOOB(ctx, &buf, msg, continued, oobTarget)
			if err != nil {
				return
			}
		}

		if prev, isContinued := s.checkRegrouping(msg, neighbors); prev != nil {
			err := ui.MessageBubbleOOB(*prev, isContinued, "outerHTML").Render(ctx, &buf)
			if err != nil {
				return
			}
		}

		s.hub.Broadcast(roomID, buf.Bytes())
	})

	return messageTree, nil
}

func (s *ChatService) SendRoomMessage(
	ctx context.Context,
	roomID string,
	author models.User,
	content string,
) error {
	userID := s.GetCurrentUserID()
	matrixSession := s.matrix.GetMatrixSession(userID)
	messageTree := matrixSession.GetMessageTree(roomID)

	return messageTree.SendMessage(ctx, content)
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
	n matrix.Neighbors,
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
