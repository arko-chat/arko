package matrix

import (
	"context"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type MatrixClient interface {
	GetProfile(ctx context.Context, userID id.UserID) (*mautrix.RespUserProfile, error)
	GetPresence(ctx context.Context, userID id.UserID) (*mautrix.RespPresence, error)
	StateEvent(ctx context.Context, roomID id.RoomID, eventType event.Type, stateKey string, output interface{}) error
	JoinedRooms(ctx context.Context) (*mautrix.RespJoinedRooms, error)
	GetAliases(ctx context.Context, roomID id.RoomID) (*mautrix.RespAliasList, error)
	CreateAlias(ctx context.Context, alias id.RoomAlias, roomID id.RoomID) (*mautrix.RespAliasCreate, error)
	State(ctx context.Context, roomID id.RoomID) (map[event.Type]map[string]*event.Event, error)
	GetAccountData(ctx context.Context, name string, output interface{}) error
	Members(ctx context.Context, roomID id.RoomID, req ...mautrix.ReqMembers) (*mautrix.RespMembers, error)
}

var _ MatrixClient = (*mautrix.Client)(nil)
