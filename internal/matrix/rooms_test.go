package matrix

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/arko-chat/arko/internal/cache"
	"github.com/arko-chat/arko/internal/models"
	"github.com/puzpuzpuz/xsync/v4"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type mockMatrixServer struct {
	server *httptest.Server
	mux    *http.ServeMux
}

func newMockMatrixServer() *mockMatrixServer {
	mux := http.NewServeMux()
	server := httptest.NewTLSServer(mux)
	return &mockMatrixServer{
		server: server,
		mux:    mux,
	}
}

func (m *mockMatrixServer) Close() {
	m.server.Close()
}

func (m *mockMatrixServer) URL() string {
	return m.server.URL
}

func newTestMatrixSessionWithServer(mockServer *mockMatrixServer) *MatrixSession {
	ctx, cancel := context.WithCancel(context.Background())

	client, _ := mautrix.NewClient(mockServer.URL(), "@test:example.com", "test-token")
	client.Client = mockServer.server.Client()

	session := &MatrixSession{
		id:                    "@test:example.com",
		logger:                slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})),
		context:               ctx,
		cancel:                cancel,
		client:                client,
		profileCache:          cache.NewDefault[models.User](),
		userCache:             cache.NewDefault[models.User](),
		aliasesCache:          cache.NewDefault[[]string](),
		roomCache:             cache.NewDefault[string](),
		channelsCache:         cache.NewDefault[[]models.Channel](),
		spacesCache:           cache.NewDefault[[]models.Space](),
		dmCache:               cache.NewDefault[[]models.User](),
		membersCache:          cache.NewDefault[[]models.User](),
		messageTrees:          xsync.NewMap[string, *MessageTree](),
		verificationListeners: xsync.NewMap[uint64, chan VerificationEvent](),
		listeners:             xsync.NewMap[uint64, chan *event.Event](),
	}
	session.keyBackupMgr = NewKeyBackupManager(session)

	return session
}

func TestGetCurrentUser_Success(t *testing.T) {
	server := newMockMatrixServer()
	defer server.Close()

	server.mux.HandleFunc("/_matrix/client/v3/profile/@test:example.com", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(mautrix.RespUserProfile{
			DisplayName: "Test User",
		})
	})

	server.mux.HandleFunc("/_matrix/client/v3/presence/@test:example.com/status", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(mautrix.RespPresence{
			CurrentlyActive: true,
		})
	})

	session := newTestMatrixSessionWithServer(server)
	defer session.Close()

	user, err := session.GetCurrentUser()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if user.ID != "@test:example.com" {
		t.Errorf("expected user ID @test:example.com, got %s", user.ID)
	}
	if user.Name != "Test User" {
		t.Errorf("expected user name 'Test User', got %s", user.Name)
	}
	if user.Status != models.StatusOnline {
		t.Errorf("expected user status online, got %s", user.Status)
	}
}

func TestGetCurrentUser_ProfileError(t *testing.T) {
	server := newMockMatrixServer()
	defer server.Close()

	server.mux.HandleFunc("/_matrix/client/v3/profile/@test:example.com", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	session := newTestMatrixSessionWithServer(server)
	defer session.Close()

	user, err := session.GetCurrentUser()
	if err != nil {
		t.Fatalf("expected no error (should fallback), got %v", err)
	}

	if user.ID != "@test:example.com" {
		t.Errorf("expected user ID @test:example.com, got %s", user.ID)
	}
	if user.Name != "test" {
		t.Errorf("expected fallback name 'test', got %s", user.Name)
	}
}

func TestGetCurrentUser_OfflinePresence(t *testing.T) {
	server := newMockMatrixServer()
	defer server.Close()

	server.mux.HandleFunc("/_matrix/client/v3/profile/@test:example.com", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(mautrix.RespUserProfile{
			DisplayName: "Test User",
		})
	})

	server.mux.HandleFunc("/_matrix/client/v3/presence/@test:example.com/status", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(mautrix.RespPresence{
			CurrentlyActive: false,
		})
	})

	session := newTestMatrixSessionWithServer(server)
	defer session.Close()

	user, err := session.GetCurrentUser()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if user.Status != models.StatusOffline {
		t.Errorf("expected user status offline, got %s", user.Status)
	}
}

func TestGetCurrentUser_Caching(t *testing.T) {
	server := newMockMatrixServer()
	defer server.Close()

	callCount := 0
	server.mux.HandleFunc("/_matrix/client/v3/profile/@test:example.com", func(w http.ResponseWriter, r *http.Request) {
		callCount++
		json.NewEncoder(w).Encode(mautrix.RespUserProfile{
			DisplayName: "Test User",
		})
	})

	session := newTestMatrixSessionWithServer(server)
	defer session.Close()

	user1, err := session.GetCurrentUser()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	user2, err := session.GetCurrentUser()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if callCount != 1 {
		t.Errorf("expected GetProfile to be called once (cached), but was called %d times", callCount)
	}

	if user1.ID != user2.ID {
		t.Error("expected same user from cache")
	}
}

func TestListSpaces_Success(t *testing.T) {
	server := newMockMatrixServer()
	defer server.Close()

	server.mux.HandleFunc("/_matrix/client/v3/joined_rooms", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(mautrix.RespJoinedRooms{
			JoinedRooms: []id.RoomID{"!space1:example.com", "!room1:example.com"},
		})
	})

	callCount := 0
	server.mux.HandleFunc("/_matrix/client/v3/rooms/", func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if r.URL.Path == "/_matrix/client/v3/rooms/!space1:example.com/state/m.room.create/" {
			json.NewEncoder(w).Encode(event.CreateEventContent{
				Type: event.RoomTypeSpace,
			})
		} else if r.URL.Path == "/_matrix/client/v3/rooms/!room1:example.com/state/m.room.create/" {
			json.NewEncoder(w).Encode(event.CreateEventContent{
				Type: event.RoomTypeDefault,
			})
		} else if r.URL.Path == "/_matrix/client/v3/rooms/!space1:example.com/state/m.room.name/" {
			json.NewEncoder(w).Encode(event.RoomNameEventContent{
				Name: "Test Space",
			})
		}
	})

	session := newTestMatrixSessionWithServer(server)
	defer session.Close()

	spaces, err := session.ListSpaces()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(spaces) != 1 {
		t.Errorf("expected 1 space, got %d", len(spaces))
		return
	}

	if spaces[0].ID != "!space1:example.com" {
		t.Errorf("expected space ID !space1:example.com, got %s", spaces[0].ID)
	}
	if spaces[0].Name != "Test Space" {
		t.Errorf("expected space name 'Test Space', got %s", spaces[0].Name)
	}
}

func TestListSpaces_JoinedRoomsError(t *testing.T) {
	server := newMockMatrixServer()
	defer server.Close()

	server.mux.HandleFunc("/_matrix/client/v3/joined_rooms", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	session := newTestMatrixSessionWithServer(server)
	defer session.Close()

	_, err := session.ListSpaces()
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestListSpaces_EmptyJoinedRooms(t *testing.T) {
	server := newMockMatrixServer()
	defer server.Close()

	server.mux.HandleFunc("/_matrix/client/v3/joined_rooms", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(mautrix.RespJoinedRooms{
			JoinedRooms: []id.RoomID{},
		})
	})

	session := newTestMatrixSessionWithServer(server)
	defer session.Close()

	spaces, err := session.ListSpaces()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(spaces) != 0 {
		t.Errorf("expected 0 spaces, got %d", len(spaces))
	}
}

func TestGetDMRoomID_Success(t *testing.T) {
	server := newMockMatrixServer()
	defer server.Close()

	server.mux.HandleFunc("/_matrix/client/v3/user/@test:example.com/account_data/m.direct", func(w http.ResponseWriter, r *http.Request) {
		dmMap := map[id.UserID][]id.RoomID{
			"@other:example.com": {"!dmroom:example.com"},
		}
		json.NewEncoder(w).Encode(dmMap)
	})

	session := newTestMatrixSessionWithServer(server)
	defer session.Close()

	roomID, err := session.GetDMRoomID("@other:example.com")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if roomID != "!dmroom:example.com" {
		t.Errorf("expected room ID !dmroom:example.com, got %s", roomID)
	}
}

func TestGetDMRoomID_NoDMRoom(t *testing.T) {
	server := newMockMatrixServer()
	defer server.Close()

	server.mux.HandleFunc("/_matrix/client/v3/user/@test:example.com/account_data/m.direct", func(w http.ResponseWriter, r *http.Request) {
		dmMap := map[id.UserID][]id.RoomID{}
		json.NewEncoder(w).Encode(dmMap)
	})

	session := newTestMatrixSessionWithServer(server)
	defer session.Close()

	_, err := session.GetDMRoomID("@other:example.com")
	if err == nil {
		t.Error("expected error for non-existent DM room, got nil")
	}
}

func TestGetDMRoomID_AccountDataError(t *testing.T) {
	server := newMockMatrixServer()
	defer server.Close()

	server.mux.HandleFunc("/_matrix/client/v3/user/@test:example.com/account_data/m.direct", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	session := newTestMatrixSessionWithServer(server)
	defer session.Close()

	_, err := session.GetDMRoomID("@other:example.com")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestListDirectMessages_Success(t *testing.T) {
	server := newMockMatrixServer()
	defer server.Close()

	server.mux.HandleFunc("/_matrix/client/v3/user/@test:example.com/account_data/m.direct", func(w http.ResponseWriter, r *http.Request) {
		dmMap := map[id.UserID][]id.RoomID{
			"@friend1:example.com": {"!dm1:example.com"},
			"@friend2:example.com": {"!dm2:example.com"},
		}
		json.NewEncoder(w).Encode(dmMap)
	})

	callCount := 0
	server.mux.HandleFunc("/_matrix/client/v3/profile/", func(w http.ResponseWriter, r *http.Request) {
		callCount++
		userID := r.URL.Path[len("/_matrix/client/v3/profile/"):]
		json.NewEncoder(w).Encode(mautrix.RespUserProfile{
			DisplayName: userID + "_display",
		})
	})

	session := newTestMatrixSessionWithServer(server)
	defer session.Close()

	friends, err := session.ListDirectMessages()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(friends) != 2 {
		t.Errorf("expected 2 friends, got %d", len(friends))
	}
}

func TestListDirectMessages_GetAccountDataError(t *testing.T) {
	server := newMockMatrixServer()
	defer server.Close()

	server.mux.HandleFunc("/_matrix/client/v3/user/@test:example.com/account_data/m.direct", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	session := newTestMatrixSessionWithServer(server)
	defer session.Close()

	_, err := session.ListDirectMessages()
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestGetChannel_Success(t *testing.T) {
	server := newMockMatrixServer()
	defer server.Close()

	server.mux.HandleFunc("/_matrix/client/v3/rooms/!channel1:example.com/state/m.room.name/", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(event.RoomNameEventContent{
			Name: "General",
		})
	})

	server.mux.HandleFunc("/_matrix/client/v3/rooms/!channel1:example.com/state/m.room.topic/", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(event.TopicEventContent{
			Topic: "General discussion",
		})
	})

	session := newTestMatrixSessionWithServer(server)
	defer session.Close()

	channel, err := session.GetChannel("!space1:example.com", "!channel1:example.com")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if channel.ID != "!channel1:example.com" {
		t.Errorf("expected channel ID !channel1:example.com, got %s", channel.ID)
	}
	if channel.Name != "General" {
		t.Errorf("expected channel name 'General', got %s", channel.Name)
	}
	if channel.Topic != "General discussion" {
		t.Errorf("expected topic 'General discussion', got %s", channel.Topic)
	}
	if channel.SpaceID != "!space1:example.com" {
		t.Errorf("expected space ID !space1:example.com, got %s", channel.SpaceID)
	}
}

func TestGetRoomMembers_Success(t *testing.T) {
	server := newMockMatrixServer()
	defer server.Close()

	server.mux.HandleFunc("/_matrix/client/v3/rooms/!room:example.com/members", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(mautrix.RespMembers{
			Chunk: []*event.Event{
				{
					Type:     event.StateMember,
					StateKey: ptr("@user1:example.com"),
					Content: event.Content{
						Parsed: &event.MemberEventContent{
							Membership:  event.MembershipJoin,
							Displayname: "User One",
						},
					},
				},
			},
		})
	})

	session := newTestMatrixSessionWithServer(server)
	defer session.Close()

	members, err := session.getRoomMembers("!room:example.com")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(members) != 1 {
		t.Errorf("expected 1 member, got %d", len(members))
	}
}

func TestGetRoomMembers_MembersError(t *testing.T) {
	server := newMockMatrixServer()
	defer server.Close()

	server.mux.HandleFunc("/_matrix/client/v3/rooms/!room:example.com/members", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	session := newTestMatrixSessionWithServer(server)
	defer session.Close()

	_, err := session.getRoomMembers("!room:example.com")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestGetUrls_WithExistingAlias(t *testing.T) {
	server := newMockMatrixServer()
	defer server.Close()

	server.mux.HandleFunc("/_matrix/client/v3/rooms/!room:example.com/aliases", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(mautrix.RespAliasList{
			Aliases: []id.RoomAlias{"#existing:example.com"},
		})
	})

	session := newTestMatrixSessionWithServer(server)
	defer session.Close()

	urls, err := session.getUrls("!room:example.com")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(urls) != 1 {
		t.Errorf("expected 1 URL, got %d", len(urls))
	}
}

func TestGetUrls_GetAliasesError(t *testing.T) {
	server := newMockMatrixServer()
	defer server.Close()

	server.mux.HandleFunc("/_matrix/client/v3/rooms/!room:example.com/aliases", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	session := newTestMatrixSessionWithServer(server)
	defer session.Close()

	_, err := session.getUrls("!room:example.com")
	if err == nil {
		t.Error("expected error when GetAliases fails, got nil")
	}
}

func TestGetSpaceChildren_StateError(t *testing.T) {
	server := newMockMatrixServer()
	defer server.Close()

	server.mux.HandleFunc("/_matrix/client/v3/rooms/!space:example.com/state", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	session := newTestMatrixSessionWithServer(server)
	defer session.Close()

	_, err := session.getSpaceChildren("!space:example.com")
	if err == nil {
		t.Error("expected error, got nil")
	}
}
