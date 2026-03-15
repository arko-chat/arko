package testutil

import (
	"context"
	"sync"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type MockClient struct {
	mu sync.RWMutex

	UserID        id.UserID
	DeviceID      id.DeviceID
	AccessToken   string
	HomeserverURL string

	WhoamiFunc           func(ctx context.Context) (*mautrix.RespWhoami, error)
	LogoutFunc           func(ctx context.Context) error
	GetLoginFlowsFunc    func(ctx context.Context) (*mautrix.RespLoginFlows, error)
	LoginFunc            func(ctx context.Context, req *mautrix.ReqLogin) (*mautrix.RespLogin, error)
	SyncFunc             func(ctx context.Context, req *mautrix.ReqSync) (*mautrix.RespSync, error)
	SendMessageEventFunc func(ctx context.Context, roomID id.RoomID, eventType event.Type, contentJSON interface{}, extra ...mautrix.ReqSendEvent) (*mautrix.RespSendEvent, error)
	GetStateEventFunc    func(ctx context.Context, roomID id.RoomID, eventType event.Type, stateKey string) (*event.Event, error)
	GetProfileFunc       func(ctx context.Context, userID id.UserID) (*mautrix.RespUserProfile, error)
	GetPresenceFunc      func(ctx context.Context, userID id.UserID) (*mautrix.RespPresence, error)
	StateEventFunc       func(ctx context.Context, roomID id.RoomID, eventType event.Type, stateKey string, output interface{}) error
	JoinedRoomsFunc      func(ctx context.Context) (*mautrix.RespJoinedRooms, error)
	GetAliasesFunc       func(ctx context.Context, roomID id.RoomID) (*mautrix.RespAliasList, error)
	CreateAliasFunc      func(ctx context.Context, alias id.RoomAlias, roomID id.RoomID) (*mautrix.RespAliasCreate, error)
	StateFunc            func(ctx context.Context, roomID id.RoomID) (map[event.Type]map[string]*event.Event, error)
	GetAccountDataFunc   func(ctx context.Context, name string, output interface{}) error
	MembersFunc          func(ctx context.Context, roomID id.RoomID, req ...mautrix.ReqMembers) (*mautrix.RespMembers, error)
}

func NewMockClient(userID string, homeserverURL string) *MockClient {
	return &MockClient{
		UserID:        id.UserID(userID),
		HomeserverURL: homeserverURL,
	}
}

func (m *MockClient) SetAccessToken(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.AccessToken = token
}

func (m *MockClient) SetDeviceID(deviceID id.DeviceID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.DeviceID = deviceID
}

func (m *MockClient) Whoami(ctx context.Context) (*mautrix.RespWhoami, error) {
	if m.WhoamiFunc != nil {
		return m.WhoamiFunc(ctx)
	}
	return &mautrix.RespWhoami{
		UserID:   m.UserID,
		DeviceID: m.DeviceID,
	}, nil
}

func (m *MockClient) Logout(ctx context.Context) error {
	if m.LogoutFunc != nil {
		return m.LogoutFunc(ctx)
	}
	return nil
}

func (m *MockClient) GetLoginFlows(ctx context.Context) (*mautrix.RespLoginFlows, error) {
	if m.GetLoginFlowsFunc != nil {
		return m.GetLoginFlowsFunc(ctx)
	}
	return &mautrix.RespLoginFlows{
		Flows: []mautrix.LoginFlow{
			{Type: mautrix.AuthTypePassword},
			{Type: mautrix.AuthTypeToken},
		},
	}, nil
}

func (m *MockClient) Login(ctx context.Context, req *mautrix.ReqLogin) (*mautrix.RespLogin, error) {
	if m.LoginFunc != nil {
		return m.LoginFunc(ctx, req)
	}
	return &mautrix.RespLogin{
		UserID:      m.UserID,
		DeviceID:    m.DeviceID,
		AccessToken: "mock-access-token",
	}, nil
}

func (m *MockClient) Sync(ctx context.Context, req *mautrix.ReqSync) (*mautrix.RespSync, error) {
	if m.SyncFunc != nil {
		return m.SyncFunc(ctx, req)
	}
	return &mautrix.RespSync{}, nil
}

func (m *MockClient) SendMessageEvent(ctx context.Context, roomID id.RoomID, eventType event.Type, contentJSON interface{}, extra ...mautrix.ReqSendEvent) (*mautrix.RespSendEvent, error) {
	if m.SendMessageEventFunc != nil {
		return m.SendMessageEventFunc(ctx, roomID, eventType, contentJSON, extra...)
	}
	return &mautrix.RespSendEvent{EventID: id.EventID("$mock-event-id")}, nil
}

func (m *MockClient) GetStateEvent(ctx context.Context, roomID id.RoomID, eventType event.Type, stateKey string) (*event.Event, error) {
	if m.GetStateEventFunc != nil {
		return m.GetStateEventFunc(ctx, roomID, eventType, stateKey)
	}
	return nil, nil
}

func (m *MockClient) GetProfile(ctx context.Context, userID id.UserID) (*mautrix.RespUserProfile, error) {
	if m.GetProfileFunc != nil {
		return m.GetProfileFunc(ctx, userID)
	}
	return &mautrix.RespUserProfile{
		DisplayName: userID.Localpart(),
	}, nil
}

func (m *MockClient) GetPresence(ctx context.Context, userID id.UserID) (*mautrix.RespPresence, error) {
	if m.GetPresenceFunc != nil {
		return m.GetPresenceFunc(ctx, userID)
	}
	return &mautrix.RespPresence{
		CurrentlyActive: true,
	}, nil
}

func (m *MockClient) StateEvent(ctx context.Context, roomID id.RoomID, eventType event.Type, stateKey string, output interface{}) error {
	if m.StateEventFunc != nil {
		return m.StateEventFunc(ctx, roomID, eventType, stateKey, output)
	}
	return nil
}

func (m *MockClient) JoinedRooms(ctx context.Context) (*mautrix.RespJoinedRooms, error) {
	if m.JoinedRoomsFunc != nil {
		return m.JoinedRoomsFunc(ctx)
	}
	return &mautrix.RespJoinedRooms{
		JoinedRooms: []id.RoomID{},
	}, nil
}

func (m *MockClient) GetAliases(ctx context.Context, roomID id.RoomID) (*mautrix.RespAliasList, error) {
	if m.GetAliasesFunc != nil {
		return m.GetAliasesFunc(ctx, roomID)
	}
	return &mautrix.RespAliasList{
		Aliases: []id.RoomAlias{},
	}, nil
}

func (m *MockClient) CreateAlias(ctx context.Context, alias id.RoomAlias, roomID id.RoomID) (*mautrix.RespAliasCreate, error) {
	if m.CreateAliasFunc != nil {
		return m.CreateAliasFunc(ctx, alias, roomID)
	}
	return &mautrix.RespAliasCreate{}, nil
}

func (m *MockClient) State(ctx context.Context, roomID id.RoomID) (map[event.Type]map[string]*event.Event, error) {
	if m.StateFunc != nil {
		return m.StateFunc(ctx, roomID)
	}
	return make(map[event.Type]map[string]*event.Event), nil
}

func (m *MockClient) GetAccountData(ctx context.Context, name string, output interface{}) error {
	if m.GetAccountDataFunc != nil {
		return m.GetAccountDataFunc(ctx, name, output)
	}
	return nil
}

func (m *MockClient) Members(ctx context.Context, roomID id.RoomID, req ...mautrix.ReqMembers) (*mautrix.RespMembers, error) {
	if m.MembersFunc != nil {
		return m.MembersFunc(ctx, roomID, req...)
	}
	return &mautrix.RespMembers{
		Chunk: []*event.Event{},
	}, nil
}

type MockCryptoHelper struct{}

func NewMockCryptoHelper() *MockCryptoHelper {
	return &MockCryptoHelper{}
}

func (m *MockCryptoHelper) Init() error  { return nil }
func (m *MockCryptoHelper) Close() error { return nil }
