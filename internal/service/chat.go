package service

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"

	"github.com/arko-chat/arko/components/ui"
	"github.com/arko-chat/arko/components/utils"
	"github.com/arko-chat/arko/internal/matrix"
	"github.com/arko-chat/arko/internal/models"
	"github.com/arko-chat/arko/internal/ws"
	"github.com/puzpuzpuz/xsync/v4"
)

type ChatService struct {
	*BaseService
	initializedTree *xsync.Map[string, struct{}]
	logger          *slog.Logger
}

func NewChatService(
	mgr matrix.ManagerClient,
	hub *ws.Hub,
	logger *slog.Logger,
) *ChatService {
	return &ChatService{
		BaseService:     NewBaseService(mgr, hub),
		initializedTree: xsync.NewMap[string, struct{}](),
		logger:          logger,
	}
}

func (s *ChatService) LoadNextMessages(roomID string, limit int) (bool, error) {
	tree, err := s.GetRoomMessageTree(roomID)
	if err != nil {
		return false, err
	}
	hasMore := tree.LoadNextMessages(s.matrix.GetContext(), limit)
	return hasMore, nil
}

func (s *ChatService) GetRoomMessageTree(roomID string) (*matrix.MessageTree, error) {
	session, err := s.GetCurrentSession()
	if err != nil {
		return nil, err
	}
	messageTree := session.GetMessageTree(roomID)

	s.initializedTree.Compute(roomID, func(str struct{}, loaded bool) (struct{}, xsync.ComputeOp) {
		if loaded {
			return str, xsync.CancelOp
		}

		messageTree.Initialize(s.matrix.GetContext())
		messageTree.Listen(s.matrix.GetContext(), func(mte matrix.MessageTreeEvent) {
			neighbors := mte.Neighbors
			msg := mte.Message

			var buf bytes.Buffer
			continued := neighbors.Prev != nil &&
				neighbors.Prev.Author.ID == msg.Author.ID &&
				utils.WithinMinutes(neighbors.Prev.Timestamp, msg.Timestamp, 5)

			switch mte.EventType {
			case matrix.AddEvent:
				if neighbors.Next != nil {
					oobTarget := "beforebegin:#msg-" + neighbors.Next.ID
					if err := renderInsertOOB(s.matrix.GetContext(), &buf, msg, continued, oobTarget); err != nil {
						s.logger.Error("render insert oob", "err", err)
						return
					}
				} else if neighbors.Prev != nil {
					oobTarget := "afterend:#msg-" + neighbors.Prev.ID
					if err := renderInsertOOB(s.matrix.GetContext(), &buf, msg, continued, oobTarget); err != nil {
						s.logger.Error("render insert oob", "err", err)
						return
					}
				} else {
					oobTarget := "beforeend:#message-list"
					if err := renderInsertOOB(s.matrix.GetContext(), &buf, msg, continued, oobTarget); err != nil {
						s.logger.Error("render insert oob", "err", err)
						return
					}
				}

			case matrix.UpdateEvent:
				oobTarget := "outerHTML:#msg-" + msg.ID
				if mte.UpdateNonce != "" {
					oobTarget = "outerHTML:#msg-" + mte.UpdateNonce
				}
				if err := renderInsertOOB(s.matrix.GetContext(), &buf, msg, continued, oobTarget); err != nil {
					s.logger.Error("render update oob", "err", err)
					return
				}

			case matrix.RemoveEvent:
				_, err := fmt.Fprintf(
					&buf,
					`<div id="msg-%s" hx-swap-oob="delete"></div>`,
					mte.Message.ID,
				)
				if err != nil {
					s.logger.Error("render remove oob", "err", err)
					return
				}
			}

			if prev, isContinued := s.checkRegrouping(msg, neighbors); prev != nil {
				if err := ui.MessageBubbleOOB(*prev, isContinued, "outerHTML").Render(s.matrix.GetContext(), &buf); err != nil {
					s.logger.Error("render regroup oob", "err", err)
					return
				}
			}

			if s.hub != nil {
				s.hub.BroadcastToRoom(roomID, buf.Bytes())
			}
		})

		return struct{}{}, xsync.UpdateOp
	})

	return messageTree, nil
}

func (s *ChatService) SendRoomMessage(roomID string, author models.User, content string) error {
	session := s.matrix.GetMatrixSession(author.ID)
	if session == nil {
		return fmt.Errorf("missing matrix session")
	}
	messageTree := session.GetMessageTree(roomID)
	return messageTree.SendMessage(content)
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
			return fmt.Errorf("render continued message oob: %w", err)
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
