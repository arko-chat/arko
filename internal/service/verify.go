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

func (s *VerificationService) IsVerified() bool {
	userID := s.GetCurrentUserID()
	return s.matrix.GetMatrixSession(userID).IsVerified()
}

func (s *VerificationService) ListenVerifyEvents(ctx context.Context) {
	userID := s.GetCurrentUserID()
	sess := s.matrix.GetMatrixSession(userID)
	if sess == nil {
		return
	}

	events, cleanup := sess.VerificationEvents(ctx)

	go func() {
		defer cleanup()
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-events:
				if !ok {
					return
				}
				msg := verificationEventToWS(ev)
				if msg == nil {
					continue
				}
				s.hub.Push(userID, msg)
			}
		}
	}()
}

func verificationEventToWS(ev matrix.VerificationEvent) []byte {
	switch ev.Type {
	case matrix.VerificationEventShowSAS:
		return ws.RedirectMessage("/verify/sas")
	case matrix.VerificationEventCancelled:
		return ws.RedirectMessage("/verify")
	case matrix.VerificationEventDone:
		return ws.RedirectMessage("/")
	case matrix.VerificationEventReady:
		if ev.Method == matrix.VerificationMethodQR {
			return ws.RedirectMessage("/verify/qr")
		}
		return ws.RedirectMessage("/verify/sas/waiting")
	case matrix.VerificationEventQRScanned:
		return ws.RedirectMessage("/verify/qr/scanned")
	}
	return nil
}

func (s *VerificationService) HasCrossSigningKeys() bool {
	userID := s.GetCurrentUserID()
	return s.matrix.HasCrossSigningKeys(userID)
}

func (s *VerificationService) GetVerificationState() *matrix.VerificationUIState {
	userID := s.GetCurrentUserID()
	return s.matrix.GetVerificationState(userID)
}

func (s *VerificationService) RequestSASVerification(ctx context.Context) error {
	userID := s.GetCurrentUserID()
	return s.matrix.RequestSASVerification(ctx, userID)
}

func (s *VerificationService) RequestQRVerification(ctx context.Context) error {
	userID := s.GetCurrentUserID()
	return s.matrix.RequestQRVerification(ctx, userID)
}

func (s *VerificationService) GetQRCodeSVG(ctx context.Context) (string, error) {
	userID := s.GetCurrentUserID()
	return s.matrix.GetQRCodeSVG(ctx, userID)
}

func (s *VerificationService) ConfirmVerification(ctx context.Context) error {
	userID := s.GetCurrentUserID()
	return s.matrix.ConfirmVerification(ctx, userID)
}

func (s *VerificationService) ConfirmQRVerification(ctx context.Context) error {
	userID := s.GetCurrentUserID()
	return s.matrix.ConfirmQRVerification(ctx, userID)
}

func (s *VerificationService) CancelVerification(ctx context.Context) error {
	userID := s.GetCurrentUserID()
	return s.matrix.CancelVerification(ctx, userID)
}

func (s *VerificationService) RecoverWithKey(ctx context.Context, key string) error {
	userID := s.GetCurrentUserID()
	return s.matrix.RecoverWithKey(ctx, userID, key)
}

func (s *VerificationService) ClearVerificationState() {
	userID := s.GetCurrentUserID()
	s.matrix.ClearVerificationState(userID)
}
