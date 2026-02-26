package service

import (
	"github.com/arko-chat/arko/internal/matrix"
	chatws "github.com/arko-chat/arko/internal/ws/chat"
	verifyws "github.com/arko-chat/arko/internal/ws/verify"
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
	chatHub *chatws.Hub,
	verifyHub *verifyws.Hub,
) *Services {
	return &Services{
		Chat:         NewChatService(mgr, chatHub),
		Friends:      NewFriendsService(mgr, chatHub),
		Spaces:       NewSpaceService(mgr, chatHub),
		User:         NewUserService(mgr, chatHub),
		Verification: NewVerificationService(mgr, verifyHub),
	}
}
