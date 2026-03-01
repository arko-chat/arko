package service

import (
	"fmt"

	"github.com/arko-chat/arko/internal/matrix"
	"github.com/arko-chat/arko/internal/models"
	"github.com/arko-chat/arko/internal/ws"
)

type SpaceService struct {
	*BaseService
}

func NewSpaceService(
	mgr *matrix.Manager,
	hub *ws.Hub,
) *SpaceService {
	return &SpaceService{
		BaseService: NewBaseService(mgr, hub),
	}
}

func (s *SpaceService) ListSpaces() ([]models.Space, error) {
	currentSession := s.matrix.GetCurrentMatrixSession()
	if currentSession == nil {
		return nil, fmt.Errorf("missing matrix session")
	}
	return currentSession.ListSpaces()
}

func (s *SpaceService) GetSpace(
	spaceID string,
) (models.SpaceDetail, error) {
	currentSession := s.matrix.GetCurrentMatrixSession()
	if currentSession == nil {
		return models.SpaceDetail{}, fmt.Errorf("missing matrix session")
	}
	return currentSession.GetSpaceDetail(spaceID)
}

func (s *SpaceService) GetChannel(
	spaceID string,
	channelID string,
) (models.Channel, error) {
	currentSession := s.matrix.GetCurrentMatrixSession()
	if currentSession == nil {
		return models.Channel{}, fmt.Errorf("missing matrix session")
	}
	return currentSession.GetChannel(spaceID, channelID)
}
