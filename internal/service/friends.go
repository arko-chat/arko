package service

import (
	"strings"

	"github.com/arko-chat/arko/internal/matrix"
	"github.com/arko-chat/arko/internal/models"
	"github.com/arko-chat/arko/internal/ws"
)

type FriendsService struct {
	*BaseService
}

func NewFriendsService(
	mgr *matrix.Manager,
	hub ws.WSHub,
) *FriendsService {
	return &FriendsService{
		BaseService: NewBaseService(mgr, hub),
	}
}

func (s *FriendsService) GetFriendRoomID(
	otherUserID string,
) (string, error) {
	userID := s.GetCurrentUserID()
	return s.matrix.GetDMRoomID(userID, otherUserID)
}

func (s *FriendsService) ListFriends() ([]models.User, error) {
	userID := s.GetCurrentUserID()
	return s.matrix.ListDirectMessages(userID)
}

func (s *FriendsService) FilterFriends(
	filter string,
) ([]models.User, error) {
	userID := s.GetCurrentUserID()
	all, err := s.matrix.ListDirectMessages(userID)
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
	query string,
) ([]models.User, error) {
	userID := s.GetCurrentUserID()
	all, err := s.matrix.ListDirectMessages(userID)
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
	otherUserID string,
) (models.User, error) {
	userID := s.GetCurrentUserID()
	return s.matrix.GetMatrixSession(userID).GetUserProfile(otherUserID)
}
