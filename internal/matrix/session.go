package matrix

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/arko-chat/arko/internal/models"
	"github.com/arko-chat/arko/internal/session"
	"github.com/puzpuzpuz/xsync/v4"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto/cryptohelper"
	"maunium.net/go/mautrix/crypto/ssss"
	"maunium.net/go/mautrix/crypto/verificationhelper"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// TODO: wrap messagetree to a matrixsession listener per chat room
type MatrixSession struct {
	sync.Mutex

	id                  string
	logger              *slog.Logger
	context             context.Context
	cancel              context.CancelFunc
	client              *mautrix.Client
	ssssMachine         *ssss.Machine
	cryptoHelper        *cryptohelper.CryptoHelper
	verificationStore   *InMemoryVerificationStore
	verificationUIState *VerificationUIState
	verificationHelper  *verificationhelper.VerificationHelper
	keyBackupMgr        *KeyBackupManager
	listeners           *xsync.Map[uint64, chan *event.Event]
	idCounter           atomic.Uint64

	messageTrees *xsync.Map[string, *MessageTree]
}

func (m *MatrixSession) Context() context.Context {
	m.Lock()
	defer m.Unlock()
	return m.context
}

func (m *MatrixSession) GetClient() *mautrix.Client {
	m.Lock()
	defer m.Unlock()
	return m.client
}

func (m *MatrixSession) GetCryptoHelper() *cryptohelper.CryptoHelper {
	m.Lock()
	defer m.Unlock()
	return m.cryptoHelper
}

func (m *MatrixSession) GetVerificationStore() *InMemoryVerificationStore {
	m.Lock()
	defer m.Unlock()
	return m.verificationStore
}

func (m *MatrixSession) GetVerificationHelper() *verificationhelper.VerificationHelper {
	m.Lock()
	defer m.Unlock()
	return m.verificationHelper
}

func (m *MatrixSession) GetVerificationUIState() *VerificationUIState {
	m.Lock()
	defer m.Unlock()
	return m.verificationUIState
}

func (m *Manager) NewMatrixSession(client *mautrix.Client, logger *slog.Logger) (*MatrixSession, error) {
	ctx, cancel := context.WithCancel(context.Background())

	dbPath := fmt.Sprintf(
		"%s/%s.db",
		m.cryptoDBPath,
		url.PathEscape(string(client.UserID)),
	)

	s, err := session.UpdateAndGet(string(client.UserID), func(s *session.Session) {
		if len(s.PickleKey) > 0 && s.LoggedIn {
			return
		}

		pickleKey, err := generatePickleKey()
		if err == nil {
			s.PickleKey = pickleKey
		}

		_ = os.Remove(dbPath)
	})

	helper, err := cryptohelper.NewCryptoHelper(
		client, s.PickleKey, dbPath,
	)
	if err != nil {
		cancel()
		return nil, err
	}

	err = helper.Init(ctx)
	if err != nil {
		helper.Close()
		cancel()
		return nil, err
	}

	store := NewInMemoryVerificationStore()
	client.Crypto = helper

	callbacks := &verificationCallbacks{
		manager: m,
		userID:  s.UserID,
		client:  client,
	}

	vh := verificationhelper.NewVerificationHelper(
		client,
		helper.Machine(),
		store,
		callbacks,
		false,
		false,
		true,
	)

	if err := vh.Init(ctx); err != nil {
		helper.Close()
		cancel()
		return nil, fmt.Errorf("init verification helper: %w", err)
	}

	client.Verification = vh

	m.startTokenRefresh(ctx, s, client)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				err := client.SyncWithContext(ctx)
				if err != nil && ctx.Err() == nil {
					m.logger.Error("sync error",
						"user", s.UserID,
						"err", err,
					)
					if errors.Is(err, mautrix.MUnknownToken) {
						_ = m.Logout(ctx, s.UserID)
						return
					}
					time.Sleep(5 * time.Second)
				}
			}
		}
	}()

	mSess := &MatrixSession{
		id:                  s.UserID,
		logger:              logger,
		context:             ctx,
		cancel:              cancel,
		client:              client,
		cryptoHelper:        helper,
		verificationStore:   store,
		verificationHelper:  vh,
		verificationUIState: &VerificationUIState{},
		ssssMachine:         ssss.NewSSSSMachine(client),
		listeners:           xsync.NewMap[uint64, chan *event.Event](),
		messageTrees:        xsync.NewMap[string, *MessageTree](),
	}

	mSess.keyBackupMgr = NewKeyBackupManager(mSess)

	err = mSess.keyBackupMgr.Init(ctx, s.UserID)
	if err != nil {
		return nil, err
	}

	mSess.initSyncHandlers()

	return mSess, nil
}

func (m *MatrixSession) initSyncHandlers() {
	syncer := m.GetClient().Syncer.(*mautrix.DefaultSyncer)

	syncer.OnEventType(
		event.EventMessage,
		func(ctx context.Context, evt *event.Event) {
			m.listeners.Range(func(id uint64, ch chan *event.Event) bool {
				ch <- evt
				return true
			})
		},
	)

	syncer.OnEventType(
		event.EventEncrypted,
		func(ctx context.Context, evt *event.Event) {
			decrypted, err := m.GetCryptoHelper().Decrypt(ctx, evt)
			if err != nil {
				return
			}
			_ = decrypted.Content.ParseRaw(decrypted.Type)
			m.listeners.Range(func(id uint64, ch chan *event.Event) bool {
				ch <- decrypted
				return true
			})
		},
	)
}

func (m *MatrixSession) listenEvents() (chan *event.Event, uint64) {
	id := m.idCounter.Add(1)
	listenCh := make(chan *event.Event, 16)
	m.listeners.Store(id, listenCh)
	return listenCh, id
}

func (m *MatrixSession) closeEventListener(id uint64) {
	if ch, ok := m.listeners.LoadAndDelete(id); ok {
		close(ch)
	}
}

func (m *MatrixSession) GetMessageTree(roomID string) *MessageTree {
	if tree, ok := m.messageTrees.Load(roomID); ok {
		return tree
	}

	tree := newMessageTree(m, roomID)
	m.messageTrees.Store(roomID, tree)

	return tree
}

func (m *MatrixSession) GetUserProfile(
	ctx context.Context,
	targetUserID string,
) (models.User, error) {
	target := id.UserID(targetUserID)
	localpart := target.Localpart()

	name := localpart
	avatar := fmt.Sprintf(
		"https://api.dicebear.com/7.x/avataaars/svg?seed=%s",
		localpart,
	)

	profile, err := m.GetClient().GetProfile(ctx, target)
	if err == nil && profile != nil {
		if profile.DisplayName != "" {
			name = profile.DisplayName
		}
		avatar = resolveContentURI(
			profile.AvatarURL, localpart, "avataaars",
		)
	}

	return models.User{
		ID:     targetUserID,
		Name:   name,
		Avatar: avatar,
		Status: models.StatusOffline,
	}, nil
}

func (m *Manager) ListJoinedRooms(
	ctx context.Context,
	userID string,
) ([]string, error) {
	client, err := m.GetClient(userID)
	if err != nil {
		return nil, err
	}

	resp, err := client.JoinedRooms(ctx)
	if err != nil {
		return nil, err
	}

	var rooms []string
	for _, r := range resp.JoinedRooms {
		rooms = append(rooms, r.String())
	}
	return rooms, nil
}

func (m *MatrixSession) IsVerified(ctx context.Context) bool {
	machine := m.GetCryptoHelper().Machine()
	if machine == nil {
		return false
	}

	device, err := machine.CryptoStore.GetDevice(
		ctx, id.UserID(m.id), machine.Client.DeviceID,
	)
	if err != nil || device == nil {
		m.logger.Error("failed to get device",
			"user", m.id,
			"error", err,
		)
		return false
	}

	trust, err := machine.ResolveTrustContext(ctx, device)
	if err != nil {
		m.logger.Error("failed to resolve trust",
			"user", m.id,
			"error", err,
		)
		return false
	}

	if trust == id.TrustStateCrossSignedTOFU {
		return true
	}

	m.logger.Debug("device not verified",
		"user", m.id,
		"trust", trust.String(),
	)
	return false
}

func (m *MatrixSession) Close() {
	if m.cancel != nil {
		m.cancel()
	}
	if m.cryptoHelper != nil {
		m.cryptoHelper.Close()
	}
	if m.verificationStore != nil {
		m.verificationStore.txns.Clear()
	}
	m.messageTrees.DeleteMatching(func(_ string, value *MessageTree) (delete bool, stop bool) {
		value.Close()
		return true, false
	})
	m.listeners.DeleteMatching(func(_ uint64, value chan *event.Event) (delete bool, stop bool) {
		close(value)
		return true, false
	})
}

func generatePickleKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("generate pickle key: %w", err)
	}
	return key, nil
}
