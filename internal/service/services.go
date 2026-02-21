package service

import (
	"github.com/arko-chat/arko/internal/matrix"
	"github.com/arko-chat/arko/internal/ws"
)

type Services struct {
	Chat         *ChatService
	Friends      *FriendsService
	Spaces       *SpaceService
	User         *UserService
	Verification *VerificationService
}

func New(
	mgr *matrix.Manager,
	hub *ws.Hub,
) *Services {
	return &Services{
		Chat:         NewChatService(mgr, hub),
		Friends:      NewFriendsService(mgr, hub),
		Spaces:       NewSpaceService(mgr, hub),
		User:         NewUserService(mgr, hub),
		Verification: NewVerificationService(mgr, hub),
	}
}
