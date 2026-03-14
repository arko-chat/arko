package matrix

import (
	"context"

	"github.com/arko-chat/arko/internal/models"
	"github.com/arko-chat/arko/internal/session"
	"maunium.net/go/mautrix"
)

type SessionClient interface {
	GetCurrentUser() (models.User, error)
	GetUserProfile(userID string) (models.User, error)
	ListSpaces() ([]models.Space, error)
	GetSpaceDetail(spaceID string) (models.SpaceDetail, error)
	GetChannel(spaceID string, channelID string) (models.Channel, error)
	ListDirectMessages() ([]models.User, error)
	GetDMRoomID(otherUserID string) (string, error)
	GetMessageTree(roomID string) *MessageTree
	IsVerified() bool
}

type VerificationClient interface {
	VerificationEvents(ctx context.Context) (<-chan VerificationEvent, func())
	WaitUntilVerified(ctx context.Context)
	HasCrossSigningKeys() bool
	GetVerificationUIState() *VerificationUIState
	RequestSASVerification(ctx context.Context) error
	RequestQRVerification(ctx context.Context) error
	GetQRCodeSVG(ctx context.Context) (string, error)
	ConfirmVerification(ctx context.Context) error
	ConfirmQRVerification(ctx context.Context) error
	CancelVerification(ctx context.Context) error
	RecoverWithKey(ctx context.Context, key string) error
}

type ManagerClient interface {
	GetCurrentUserID() string
	GetCurrentMatrixSession() SessionClient
	GetMatrixSession(userID string) SessionClient
	GetContext() context.Context
	HasCrossSigningKeys(userID string) bool
	GetVerificationState(userID string) *VerificationUIState
	RequestSASVerification(ctx context.Context, userID string) error
	RequestQRVerification(ctx context.Context, userID string) error
	GetQRCodeSVG(ctx context.Context, userID string) (string, error)
	ConfirmVerification(ctx context.Context, userID string) error
	ConfirmQRVerification(ctx context.Context, userID string) error
	CancelVerification(ctx context.Context, userID string) error
	RecoverWithKey(ctx context.Context, userID string, key string) error
	ClearVerificationState(userID string)
	GetSupportedAuthTypes(ctx context.Context, creds models.LoginCredentials) ([]mautrix.AuthType, error)
	Login(ctx context.Context, creds models.LoginCredentials) (*session.Session, error)
	Logout(ctx context.Context, userID string) error
}
