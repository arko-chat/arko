package service

import (
	"bytes"
	"context"

	"github.com/a-h/templ"

	"github.com/arko-chat/arko/internal/matrix"
	"github.com/arko-chat/arko/internal/models"
	"github.com/arko-chat/arko/internal/ws"
)

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

func (s *ChatService) SendRoomMessage(
	ctx context.Context,
	roomID string,
	author models.User,
	content string,
	renderer func(models.Message) templ.Component,
) error {
	userID := s.GetCurrentUserID()
	err := s.matrix.SendMessage(ctx, userID, roomID, content)
	if err != nil {
		return err
	}

	msg := models.Message{
		Content: content,
		Author:  author,
	}

	var inner bytes.Buffer
	if err := renderer(msg).Render(ctx, &inner); err != nil {
		return err
	}

	var payload bytes.Buffer
	payload.WriteString(`<div id="message-list" hx-swap-oob="beforeend">`)
	payload.Write(inner.Bytes())
	payload.WriteString(`</div>`)

	s.hub.Broadcast(roomID, payload.Bytes())
	return nil
}
