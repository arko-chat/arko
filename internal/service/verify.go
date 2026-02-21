package service

import (
	"context"

	"github.com/arko-chat/arko/internal/matrix"
	"github.com/arko-chat/arko/internal/ws"
)

type VerificationService struct {
	*BaseService
}

func NewVerificationService(
	mgr *matrix.Manager,
	hub *ws.Hub,
) *VerificationService {
	return &VerificationService{
		BaseService: NewBaseService(mgr, hub),
	}
}

func (s *VerificationService) IsVerified(ctx context.Context) bool {
	userID := s.GetCurrentUserID()
	return s.matrix.IsVerified(ctx, userID)
}

func (s *VerificationService) HasCrossSigningKeys() bool {
	userID := s.GetCurrentUserID()
	return s.matrix.HasCrossSigningKeys(userID)
}

func (s *VerificationService) GetVerificationState() *matrix.VerificationUIState {
	userID := s.GetCurrentUserID()
	return s.matrix.GetVerificationState(userID)
}

func (s *VerificationService) ConfirmVerification(
	ctx context.Context,
) error {
	userID := s.GetCurrentUserID()
	return s.matrix.ConfirmVerification(ctx, userID)
}

func (s *VerificationService) CancelVerification(
	ctx context.Context,
) error {
	userID := s.GetCurrentUserID()
	return s.matrix.CancelVerification(ctx, userID)
}

func (s *VerificationService) ClearVerificationState() {
	userID := s.GetCurrentUserID()
	s.matrix.ClearVerificationState(userID)
}
