package service

import (
	"context"
	"strings"

	"github.com/arko-chat/arko/internal/matrix"
	"github.com/arko-chat/arko/internal/models"
	"github.com/arko-chat/arko/internal/ws"
	"github.com/puzpuzpuz/xsync/v4"
)

type FriendsService struct {
	*BaseService
	messages *xsync.Map[string, *models.MessageTree]
}

func NewFriendsService(
	mgr *matrix.Manager,
	hub *ws.Hub,
) *FriendsService {
	return &FriendsService{
		BaseService: NewBaseService(mgr, hub),
	}
}

func (s *FriendsService) GetFriendMessages(
	ctx context.Context,
	otherUserID string,
) (*models.MessageTree, error) {
	userID := s.GetCurrentUserID()
	messageTree, _ := s.messages.LoadOrStore(otherUserID, models.NewMessageTree())

	roomID, err := s.matrix.GetDMRoomID(ctx, userID, otherUserID)
	if err != nil {
		return nil, err
	}

	if messageTree.Len() == 0 {
		messages, _ := s.matrix.GetRoomMessages(ctx, userID, roomID, "", "", 50)
		for _, message := range messages {
			messageTree.Set(message)
		}
	}

	return messageTree, nil
}

func (s *FriendsService) GetFriendRoomID(
	ctx context.Context,
	otherUserID string,
) (string, error) {
	userID := s.GetCurrentUserID()
	return s.matrix.GetDMRoomID(ctx, userID, otherUserID)
}

func (s *FriendsService) ListFriends(
	ctx context.Context,
) ([]models.User, error) {
	userID := s.GetCurrentUserID()
	return s.matrix.ListDirectMessages(ctx, userID)
}

func (s *FriendsService) FilterFriends(
	ctx context.Context,
	filter string,
) ([]models.User, error) {
	userID := s.GetCurrentUserID()
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

func (s *FriendsService) SearchFriends(
	ctx context.Context,
	query string,
) ([]models.User, error) {
	userID := s.GetCurrentUserID()
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

func (s *FriendsService) GetFriend(
	ctx context.Context,
	otherUserID string,
) (models.User, error) {
	userID := s.GetCurrentUserID()
	return s.matrix.GetUserProfile(ctx, userID, otherUserID)
}
