package service

import (
	"context"

	"github.com/arko-chat/arko/internal/matrix"
	"github.com/arko-chat/arko/internal/models"
	"github.com/arko-chat/arko/internal/session"
	"github.com/arko-chat/arko/internal/ws"
	"maunium.net/go/mautrix"
)

type UserService struct {
	*BaseService
}

func NewUserService(
	mgr *matrix.Manager,
	hub ws.WSHub,
) *UserService {
	return &UserService{
		BaseService: NewBaseService(mgr, hub),
	}
}

func (s *UserService) GetSupportedAuthTypes(
	ctx context.Context,
	creds models.LoginCredentials,
) ([]mautrix.AuthType, error) {
	return s.matrix.GetSupportedAuthTypes(ctx, creds)
}

func (s *UserService) Login(
	ctx context.Context,
	creds models.LoginCredentials,
) (*session.Session, error) {
	return s.matrix.Login(ctx, creds)
}

func (s *UserService) Logout(
	ctx context.Context,
) error {
	userID := s.GetCurrentUserID()
	return s.matrix.Logout(ctx, userID)
}
