package service

import (
	"context"

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

func (s *SpaceService) ListSpaces(
	ctx context.Context,
) ([]models.Space, error) {
	userID := s.GetCurrentUserID()
	return s.matrix.ListSpaces(ctx, userID)
}

func (s *SpaceService) GetSpace(
	ctx context.Context,
	spaceID string,
) (models.SpaceDetail, error) {
	userID := s.GetCurrentUserID()
	return s.matrix.GetSpaceDetail(ctx, userID, spaceID)
}

func (s *SpaceService) GetChannel(
	ctx context.Context,
	spaceID string,
	channelID string,
) (models.Channel, error) {
	userID := s.GetCurrentUserID()
	return s.matrix.GetChannel(ctx, userID, spaceID, channelID)
}
