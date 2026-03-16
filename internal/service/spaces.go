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
	mgr matrix.ManagerClient,
	hub *ws.Hub,
) *SpaceService {
	return &SpaceService{
		BaseService: NewBaseService(mgr, hub),
	}
}

func (s *SpaceService) ListSpaces() ([]models.Space, error) {
	session, err := s.GetCurrentSession()
	if err != nil {
		return nil, err
	}
	return session.ListSpaces()
}

func (s *SpaceService) GetSpace(spaceID string) (models.SpaceDetail, error) {
	session, err := s.GetCurrentSession()
	if err != nil {
		return models.SpaceDetail{}, err
	}
	return session.GetSpaceDetail(spaceID)
}

func (s *SpaceService) GetChannel(spaceID string, channelID string) (models.Channel, error) {
	session, err := s.GetCurrentSession()
	if err != nil {
		return models.Channel{}, err
	}
	return session.GetChannel(spaceID, channelID)
}
