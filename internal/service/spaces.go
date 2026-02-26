package service

import (
	"github.com/arko-chat/arko/internal/matrix"
	"github.com/arko-chat/arko/internal/models"
	"github.com/arko-chat/arko/internal/ws"
)

type SpaceService struct {
	*BaseService
}

func NewSpaceService(
	mgr *matrix.Manager,
	hub ws.WSHub,
) *SpaceService {
	return &SpaceService{
		BaseService: NewBaseService(mgr, hub),
	}
}

func (s *SpaceService) ListSpaces() ([]models.Space, error) {
	userID := s.GetCurrentUserID()
	return s.matrix.ListSpaces(userID)
}

func (s *SpaceService) GetSpace(
	spaceID string,
) (models.SpaceDetail, error) {
	userID := s.GetCurrentUserID()
	return s.matrix.GetSpaceDetail(userID, spaceID)
}

func (s *SpaceService) GetChannel(
	spaceID string,
	channelID string,
) (models.Channel, error) {
	userID := s.GetCurrentUserID()
	return s.matrix.GetChannel(userID, spaceID, channelID)
}
