package service

import "context"

func (s *ChatService) IsVerified(userID string) bool {
	return s.matrix.IsVerified(userID)
}

func (s *ChatService) HasCrossSigningKeys(userID string) bool {
	return s.matrix.HasCrossSigningKeys(userID)
}

func (s *ChatService) SetupCrossSigning(
	ctx context.Context,
	userID string,
	password string,
) error {
	return s.matrix.SetupCrossSigningInteractive(
		ctx, userID, password,
	)
}

func (s *ChatService) GetVerificationState(
	userID string,
) interface{} {
	return s.matrix.GetVerificationState(userID)
}

func (s *ChatService) ConfirmVerification(
	ctx context.Context,
	userID string,
) error {
	return s.matrix.ConfirmVerification(ctx, userID)
}

func (s *ChatService) CancelVerification(
	ctx context.Context,
	userID string,
) error {
	return s.matrix.CancelVerification(ctx, userID)
}

func (s *ChatService) ClearVerificationState(userID string) {
	s.matrix.ClearVerificationState(userID)
}
