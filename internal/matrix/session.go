package matrix

import (
	"context"
	cryptorand "crypto/rand"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/arko-chat/arko/internal/cache"
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

type MatrixSession struct {
	sync.Mutex

	id                    string
	manager               *Manager
	seenAsVerified        atomic.Bool
	logger                *slog.Logger
	context               context.Context
	cancel                context.CancelFunc
	client                *mautrix.Client
	ssssMachine           *ssss.Machine
	cryptoHelper          *cryptohelper.CryptoHelper
	verificationStore     *InMemoryVerificationStore
	verificationUIState   *VerificationUIState
	verificationHelper    *verificationhelper.VerificationHelper
	verificationListeners *xsync.Map[uint64, chan VerificationEvent]
	verificationIdCounter atomic.Uint64
	keyBackupMgr          *KeyBackupManager
	listeners             *xsync.Map[uint64, chan *event.Event]
	idCounter             atomic.Uint64

	crossSigningEvent chan struct{}

	profileCache  *cache.Cache[models.User]
	verifiedCache *cache.Cache[bool]

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

func (m *Manager) NewMatrixSession(ctx context.Context, client *mautrix.Client, logger *slog.Logger) (*MatrixSession, error) {
	ctx, cancel := context.WithCancel(ctx)

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
		backoff := 5 * time.Second
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
					jitter := time.Duration(rand.N(2 * time.Second))
					select {
					case <-time.After(backoff + jitter):
					case <-ctx.Done():
						return
					}
				} else {
					backoff = 5 * time.Second
				}
			}
		}
	}()

	mSess := &MatrixSession{
		id:                    s.UserID,
		manager:               m,
		logger:                logger,
		context:               ctx,
		cancel:                cancel,
		client:                client,
		cryptoHelper:          helper,
		verificationStore:     store,
		verificationHelper:    vh,
		verificationUIState:   &VerificationUIState{},
		verificationListeners: xsync.NewMap[uint64, chan VerificationEvent](),
		ssssMachine:           ssss.NewSSSSMachine(client),
		listeners:             xsync.NewMap[uint64, chan *event.Event](),
		messageTrees:          xsync.NewMap[string, *MessageTree](),
		profileCache:          cache.NewDefault[models.User](),
		verifiedCache:         cache.New[bool](time.Minute * 30),
		crossSigningEvent:     make(chan struct{}, 2),
	}

	mSess.keyBackupMgr = NewKeyBackupManager(mSess)

	err = mSess.keyBackupMgr.Init(ctx, s.UserID)
	if err != nil {
		return nil, err
	}

	mSess.initSyncHandlers()

	return mSess, nil
}

func (m *MatrixSession) broadcastVerificationEvent(evt VerificationEvent) {
	m.verificationListeners.Range(func(_ uint64, ch chan VerificationEvent) bool {
		ch <- evt
		return true
	})
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
		event.EventRedaction,
		func(ctx context.Context, evt *event.Event) {
			m.listeners.Range(func(id uint64, ch chan *event.Event) bool {
				ch <- evt
				return true
			})
		},
	)

	syncer.OnEventType(
		event.EventReaction,
		func(ctx context.Context, evt *event.Event) {
			m.listeners.Range(func(id uint64, ch chan *event.Event) bool {
				ch <- evt
				return true
			})
		},
	)

	syncer.OnEventType(
		event.EventSticker,
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

	syncer.OnEventType(event.StateMember, func(ctx context.Context, evt *event.Event) {
		m.manager.membersCache.Invalidate("grm:" + string(evt.RoomID))
		m.profileCache.Invalidate("gup:" + evt.GetStateKey())
		m.manager.dmCache.Invalidate("ldm:" + m.id)
	})

	syncer.OnEventType(event.StateSpaceChild, func(ctx context.Context, evt *event.Event) {
		m.manager.channelsCache.Invalidate("gsc:" + evt.RoomID.String())
	})

	syncer.OnEventType(event.AccountDataDirectChats, func(ctx context.Context, evt *event.Event) {
		m.manager.dmCache.Invalidate("ldm:" + m.id)
	})

	syncer.OnEventType(event.EphemeralEventPresence, func(ctx context.Context, evt *event.Event) {
		m.profileCache.Invalidate("gup:" + evt.Sender.String())
		if evt.Sender.String() == m.id {
			m.manager.userCache.Invalidate("gcu:" + m.id)
		}
	})

	syncer.OnEventType(
		event.AccountDataCrossSigningSelf,
		func(ctx context.Context, evt *event.Event) {
			machine := m.GetCryptoHelper().Machine()
			if machine == nil {
				return
			}

			device, err := machine.CryptoStore.GetDevice(
				ctx, id.UserID(m.id), machine.Client.DeviceID,
			)
			if err != nil || device == nil {
				return
			}

			if err := machine.SignOwnDevice(ctx, device); err != nil {
				m.logger.Error("failed to self-sign device after key arrival",
					"user", m.id,
					"error", err,
				)
				return
			}

			m.logger.Info("self-signed device after cross-signing key arrival", "user", m.id)
			m.verifiedCache.Invalidate("iv:" + m.id)
			if m.crossSigningEvent != nil {
				m.crossSigningEvent <- struct{}{}
			}
		},
	)
}

func (m *MatrixSession) listenEvents() (chan *event.Event, uint64) {
	id := m.idCounter.Add(1)
	listenCh := make(chan *event.Event, 16)
	m.listeners.Store(id, listenCh)
	return listenCh, id
}

func (m *MatrixSession) VerificationEvents(ctx context.Context) (<-chan VerificationEvent, func()) {
	id := m.verificationIdCounter.Add(1)
	ch := make(chan VerificationEvent, 16)
	m.verificationListeners.Store(id, ch)

	cancel := func() {
		if ch, ok := m.verificationListeners.LoadAndDelete(id); ok {
			close(ch)
		}
	}

	go func() {
		<-ctx.Done()
		cancel()
	}()

	return ch, cancel
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

	var encEvt event.EncryptionEventContent
	err := m.GetClient().StateEvent(
		m.Context(), id.RoomID(roomID), event.StateEncryption, "", &encEvt,
	)

	tree := newMessageTree(m, roomID)
	tree.isEncrypted = err == nil && encEvt.Algorithm != ""

	m.messageTrees.Store(roomID, tree)

	return tree
}

func (m *MatrixSession) GetUserProfile(
	targetUserID string,
) (models.User, error) {
	return m.profileCache.Get("gup:"+targetUserID, func() (models.User, error) {
		ctx := m.context
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
			avatar = resolveContentURI(profile.AvatarURL, localpart, "avataaars")
		}

		user := models.User{
			ID:     targetUserID,
			Name:   name,
			Avatar: avatar,
			Status: models.StatusOffline,
		}
		presence, err := m.GetClient().GetPresence(ctx, target)
		if err == nil {
			if presence.CurrentlyActive {
				user.Status = models.StatusOnline
			} else {
				user.Status = models.StatusOffline
			}
		}

		return user, nil
	})
}

func (m *MatrixSession) IsVerified() bool {
	check := func() (bool, error) {
		ctx := m.context
		machine := m.GetCryptoHelper().Machine()
		if machine == nil {
			return false, fmt.Errorf("machine is nil")
		}

		device, err := machine.CryptoStore.GetDevice(
			ctx, id.UserID(m.id), machine.Client.DeviceID,
		)
		if err != nil || device == nil {
			m.logger.Error("failed to get device",
				"user", m.id,
				"error", err,
			)
			return false, err
		}

		trust, err := machine.ResolveTrustContext(ctx, device)
		if err != nil {
			m.logger.Error("failed to resolve trust",
				"user", m.id,
				"error", err,
			)
			return false, err
		}

		if trust >= id.TrustStateCrossSignedTOFU {
			return true, nil
		}

		m.logger.Debug("device not verified",
			"user", m.id,
			"trust", trust.String(),
		)
		return false, fmt.Errorf("trust is not considered valid: %s", trust.String())
	}

	if m.seenAsVerified.Load() {
		cached, _ := m.verifiedCache.Get("iv:"+m.id, check)
		return cached
	}

	verified, _ := check()
	m.seenAsVerified.Store(verified)

	return verified
}

func (m *MatrixSession) WaitUntilVerified(ctx context.Context) error {
	if m.IsVerified() {
		return nil
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-m.crossSigningEvent:
			if m.IsVerified() {
				return nil
			}
		case <-ticker.C:
			if m.IsVerified() {
				return nil
			}
		}
	}
}

func (m *MatrixSession) Close() {
	if m.cancel != nil {
		m.cancel()
	}
	if m.crossSigningEvent != nil {
		close(m.crossSigningEvent)
		m.crossSigningEvent = nil
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
	m.verificationListeners.DeleteMatching(func(_ uint64, value chan VerificationEvent) (delete bool, stop bool) {
		close(value)
		return true, false
	})
}

func generatePickleKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(cryptorand.Reader, key); err != nil {
		return nil, fmt.Errorf("generate pickle key: %w", err)
	}
	return key, nil
}
