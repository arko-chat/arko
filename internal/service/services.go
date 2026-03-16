package service

import (
	"log/slog"

	"github.com/arko-chat/arko/internal/matrix"
	"github.com/arko-chat/arko/internal/ws"
)

type Services struct {
	Chat         *ChatService
	Friends      *FriendsService
	Spaces       *SpaceService
	User         *UserService
	Verification *VerificationService
	WebView      *WebViewService
}

func New(mgr *matrix.Manager, wsHub *ws.Hub, logger *slog.Logger) *Services {
	return &Services{
		Chat:         NewChatService(mgr, wsHub, logger),
		Friends:      NewFriendsService(mgr, wsHub),
		Spaces:       NewSpaceService(mgr, wsHub),
		User:         NewUserService(mgr, wsHub),
		Verification: NewVerificationService(mgr, wsHub),
		WebView:      NewWebViewService(mgr, wsHub),
	}
}
