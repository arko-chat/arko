package service

import (
	"bytes"
	"context"
	"strings"

	"github.com/a-h/templ"

	"github.com/arko-chat/arko/internal/matrix"
	"github.com/arko-chat/arko/internal/models"
	"github.com/arko-chat/arko/internal/session"
	"github.com/arko-chat/arko/internal/ws"
)

type ChatService struct {
	matrix *matrix.Manager
	hub    *ws.Hub
}

func NewChatService(
	mgr *matrix.Manager,
	hub *ws.Hub,
) *ChatService {
	return &ChatService{
		matrix: mgr,
		hub:    hub,
	}
}

func (s *ChatService) Hub() *ws.Hub {
	return s.hub
}

func (s *ChatService) Matrix() *matrix.Manager {
	return s.matrix
}

func (s *ChatService) GetCurrentUser(
	ctx context.Context,
	userID string,
) (models.User, error) {
	return s.matrix.GetCurrentUser(ctx, userID)
}

func (s *ChatService) ListSpaces(
	ctx context.Context,
	userID string,
) ([]models.Space, error) {
	return s.matrix.ListSpaces(ctx, userID)
}

func (s *ChatService) GetSpaceDetail(
	ctx context.Context,
	userID string,
	spaceID string,
) (models.SpaceDetail, error) {
	return s.matrix.GetSpaceDetail(ctx, userID, spaceID)
}

func (s *ChatService) GetChannel(
	ctx context.Context,
	userID string,
	spaceID string,
	channelID string,
) (models.Channel, error) {
	return s.matrix.GetChannel(ctx, userID, spaceID, channelID)
}

func (s *ChatService) GetChannelMessages(
	ctx context.Context,
	userID string,
	channelID string,
) ([]models.Message, error) {
	return s.matrix.GetRoomMessages(ctx, userID, channelID, 50)
}

func (s *ChatService) ListFriends(
	ctx context.Context,
	userID string,
) ([]models.User, error) {
	return s.matrix.ListDirectMessages(ctx, userID)
}

func (s *ChatService) FilterFriends(
	ctx context.Context,
	userID string,
	filter string,
) ([]models.User, error) {
	all, err := s.matrix.ListDirectMessages(ctx, userID)
	if err != nil {
		return nil, err
	}

	if filter == "all" {
		return all, nil
	}

	var out []models.User
	for _, f := range all {
		switch filter {
		case "online":
			if f.Status == models.StatusOnline {
				out = append(out, f)
			}
		case "pending", "blocked":
		default:
			out = append(out, f)
		}
	}
	return out, nil
}

func (s *ChatService) SearchFriends(
	ctx context.Context,
	userID string,
	query string,
) ([]models.User, error) {
	all, err := s.matrix.ListDirectMessages(ctx, userID)
	if err != nil {
		return nil, err
	}

	q := strings.ToLower(query)
	var out []models.User
	for _, f := range all {
		if q == "" || strings.Contains(strings.ToLower(f.Name), q) {
			out = append(out, f)
		}
	}
	return out, nil
}

func (s *ChatService) GetFriend(
	ctx context.Context,
	userID string,
	otherUserID string,
) (models.User, error) {
	return s.matrix.GetUserProfile(ctx, userID, otherUserID)
}

func (s *ChatService) GetDMMessages(
	ctx context.Context,
	userID string,
	otherUserID string,
) ([]models.Message, error) {
	roomID, err := s.matrix.GetDMRoomID(ctx, userID, otherUserID)
	if err != nil {
		return nil, err
	}
	return s.matrix.GetRoomMessages(ctx, userID, roomID, 50)
}

func (s *ChatService) GetDMRoomID(
	ctx context.Context,
	userID string,
	otherUserID string,
) (string, error) {
	return s.matrix.GetDMRoomID(ctx, userID, otherUserID)
}

func (s *ChatService) SendRoomMessage(
	ctx context.Context,
	roomID string,
	userID string,
	author models.User,
	content string,
	renderer func(models.Message) templ.Component,
) error {
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

func (s *ChatService) Login(
	ctx context.Context,
	creds models.LoginCredentials,
) (*session.Session, error) {
	return s.matrix.Login(ctx, creds)
}

func (s *ChatService) RestoreSession(
	sess session.Session,
) error {
	return s.matrix.RestoreSession(sess)
}

func (s *ChatService) Logout(
	ctx context.Context,
	userID string,
) error {
	return s.matrix.Logout(ctx, userID)
}
