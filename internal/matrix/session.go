package matrix

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/arko-chat/arko/internal/session"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto/cryptohelper"
	"maunium.net/go/mautrix/crypto/ssss"
	"maunium.net/go/mautrix/crypto/verificationhelper"
	"maunium.net/go/mautrix/event"
)

type MatrixSession struct {
	sync.Mutex

	context             context.Context
	cancel              context.CancelFunc
	client              *mautrix.Client
	ssssMachine         *ssss.Machine
	cryptoHelper        *cryptohelper.CryptoHelper
	verificationStore   *InMemoryVerificationStore
	verificationUIState *VerificationUIState
	verificationHelper  *verificationhelper.VerificationHelper
	keyBackupMgr        *KeyBackupManager
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

func (m *Manager) NewMatrixSession(client *mautrix.Client) (*MatrixSession, error) {
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

	syncer := client.Syncer.(*mautrix.DefaultSyncer)

	syncer.OnEventType(
		event.EventMessage,
		func(ctx context.Context, evt *event.Event) {
			if sentBySelf := m.sentMsgIds.Remove(evt.ID.String()); sentBySelf {
				return
			}
			html := m.eventToHTML(client, evt)
			if html == nil {
				return
			}
			rawRoomID := evt.RoomID.String()
			m.hub.Broadcast(rawRoomID, html)
		},
	)

	helper.CustomPostDecrypt = func(
		ctx context.Context, evt *event.Event,
	) {
		if evt.Type != event.EventMessage {
			return
		}
		if evt.Sender.String() == s.UserID {
			return
		}
		html := m.eventToHTML(client, evt)
		if html == nil {
			return
		}
		rawRoomID := evt.RoomID.String()
		m.hub.Broadcast(rawRoomID, html)
	}

	syncer.OnEventType(
		event.EventEncrypted,
		helper.HandleEncrypted,
	)

	client.Syncer = syncer

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
		context:             ctx,
		cancel:              cancel,
		client:              client,
		cryptoHelper:        helper,
		verificationStore:   store,
		verificationHelper:  vh,
		verificationUIState: &VerificationUIState{},
		ssssMachine:         ssss.NewSSSSMachine(client),
	}

	mSess.keyBackupMgr = NewKeyBackupManager(mSess)

	err = mSess.keyBackupMgr.Init(ctx, s.UserID)
	if err != nil {
		return nil, err
	}

	return mSess, nil
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
}

func generatePickleKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("generate pickle key: %w", err)
	}
	return key, nil
}
