package service

import (
	"context"

	"github.com/arko-chat/arko/internal/matrix"
	"github.com/arko-chat/arko/internal/models"
	"github.com/arko-chat/arko/internal/ws"
	"github.com/puzpuzpuz/xsync/v4"
)

type SpaceService struct {
	*BaseService
	messages *xsync.Map[string, *models.MessageTree]
}

func NewSpaceService(
	mgr *matrix.Manager,
	hub *ws.Hub,
) *SpaceService {
	return &SpaceService{
		BaseService: NewBaseService(mgr, hub),
		messages:    xsync.NewMap[string, *models.MessageTree](),
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

func (s *SpaceService) ListenForChannelMessages(ctx context.Context, channelID string) chan models.Message {
	return nil
}

func (s *SpaceService) GetChannelMessages(
	ctx context.Context,
	channelID string,
) (*models.MessageTree, error) {
	userID := s.GetCurrentUserID()
	messageTree, _ := s.messages.LoadOrStore(channelID, models.NewMessageTree())

	if messageTree.Len() == 0 {
		messages, _ := s.matrix.GetRoomMessages(ctx, userID, channelID, "", "", 50)
		for _, message := range messages {
			messageTree.Set(message)
		}
	}

	return messageTree, nil
}
