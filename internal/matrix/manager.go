// TODO: fix message order
// TODO: seems like encrypted messages are after first view?
// TODO: figure out a better way to store crypto, should persist across restarts
// TODO: consider using matrix js sdk for frontend crypto?
// TODO: fix messages from other clients not received in real time
// TODO: aggressive caching for a more responsive experience
// TODO: avatars not working

package matrix

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto/cryptohelper"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"github.com/arko-chat/arko/components/ui"
	"github.com/arko-chat/arko/internal/credentials"
	"github.com/arko-chat/arko/internal/models"
	"github.com/arko-chat/arko/internal/ws"
)

var ErrNoClient = errors.New("no client for user")
var ErrNotVerified = errors.New(
	"device not verified: cross-signing setup required",
)

type Manager struct {
	mu                 sync.RWMutex
	clients            map[string]*mautrix.Client
	cryptoHelpers      map[string]*cryptohelper.CryptoHelper
	hub                *ws.Hub
	logger             *slog.Logger
	cancels            map[string]context.CancelFunc
	cryptoDBPath       string
	pickleKey          []byte
	verificationStates map[string]*VerificationState
	sasSessions        map[string]*sasSession
	verifiedUsers      map[string]bool
	recoveryKeys       map[string]string
}

func NewManager(
	hub *ws.Hub,
	logger *slog.Logger,
	cryptoDBPath string,
	pickleKey []byte,
) *Manager {
	return &Manager{
		clients:            make(map[string]*mautrix.Client),
		cryptoHelpers:      make(map[string]*cryptohelper.CryptoHelper),
		cancels:            make(map[string]context.CancelFunc),
		hub:                hub,
		logger:             logger,
		cryptoDBPath:       cryptoDBPath,
		pickleKey:          pickleKey,
		verificationStates: make(map[string]*VerificationState),
		sasSessions:        make(map[string]*sasSession),
		verifiedUsers:      make(map[string]bool),
		recoveryKeys:       make(map[string]string),
	}
}

func (m *Manager) MarkVerified(userID string) {
	m.mu.Lock()
	m.verifiedUsers[userID] = true
	m.mu.Unlock()

	if err := credentials.StoreVerified(userID, true); err != nil {
		m.logger.Warn("failed to store verified flag in keyring",
			"user", userID,
			"err", err,
		)
	}
}

func (m *Manager) IsVerified(userID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.verifiedUsers[userID] {
		return true
	}

	if credentials.LoadVerified(userID) {
		m.verifiedUsers[userID] = true
		return true
	}

	helper, ok := m.cryptoHelpers[userID]
	client := m.clients[userID]

	if !ok || helper == nil || client == nil {
		return false
	}

	machine := helper.Machine()
	if machine == nil {
		return false
	}

	ctx := context.Background()

	pubkeys := machine.GetOwnCrossSigningPublicKeys(ctx)
	if pubkeys == nil {
		return false
	}

	device, err := machine.CryptoStore.GetDevice(
		ctx, id.UserID(userID), client.DeviceID,
	)
	if err != nil || device == nil {
		return false
	}

	if device.Trust == id.TrustStateCrossSignedVerified {
		m.verifiedUsers[userID] = true
		_ = credentials.StoreVerified(userID, true)
		return true
	}

	return false
}

func (m *Manager) HasClient(userID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.clients[userID]
	return ok
}

func (m *Manager) GetClient(userID string) (*mautrix.Client, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	client, ok := m.clients[userID]
	if !ok {
		return nil, ErrNoClient
	}
	return client, nil
}

func (m *Manager) GetVerificationState(
	userID string,
) *VerificationState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.verificationStates[userID]
}

func (m *Manager) Login(
	ctx context.Context,
	creds models.LoginCredentials,
) (*models.MatrixSession, error) {
	homeserver := creds.Homeserver
	if !strings.HasPrefix(homeserver, "http") {
		homeserver = "https://" + homeserver
	}

	client, err := mautrix.NewClient(homeserver, "", "")
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}

	loginReq := &mautrix.ReqLogin{
		Type: mautrix.AuthTypePassword,
		Identifier: mautrix.UserIdentifier{
			Type: mautrix.IdentifierTypeUser,
			User: creds.Username,
		},
		Password:                 creds.Password,
		InitialDeviceDisplayName: "Arko Web Client",
		StoreCredentials:         true,
	}

	if creds.DeviceID != "" {
		loginReq.DeviceID = id.DeviceID(creds.DeviceID)
	}

	resp, err := client.Login(ctx, loginReq)
	if err != nil {
		return nil, fmt.Errorf("login: %w", err)
	}

	session := &models.MatrixSession{
		Homeserver:  homeserver,
		UserID:      resp.UserID.String(),
		AccessToken: resp.AccessToken,
		DeviceID:    resp.DeviceID.String(),
	}

	meta := credentials.SessionMetadata{
		Homeserver: session.Homeserver,
		UserID:     session.UserID,
		DeviceID:   session.DeviceID,
	}
	if err := credentials.StoreSession(meta, session.AccessToken); err != nil {
		m.logger.Error("failed to store session in keyring",
			"user", session.UserID,
			"err", err,
		)
	}
	_ = credentials.AddKnownUser(session.UserID)

	m.mu.Lock()
	m.clients[session.UserID] = client
	m.mu.Unlock()

	dbPath := fmt.Sprintf(
		"%s/%s.db",
		m.cryptoDBPath,
		url.PathEscape(session.UserID),
	)

	if err := m.setupCrypto(
		ctx, session.UserID, dbPath, m.pickleKey,
	); err != nil {
		m.logger.Error("crypto setup failed",
			"user", session.UserID,
			"err", err,
		)
	}

	m.startSync(session.UserID, client)
	return session, nil
}

func (m *Manager) RestoreAllSessions() {
	users := credentials.GetKnownUsers()
	for _, userID := range users {
		meta, token, err := credentials.LoadSession(userID)
		if err != nil {
			m.logger.Warn("skipping stored session",
				"user", userID,
				"err", err,
			)
			continue
		}

		err = m.RestoreSession(models.MatrixSession{
			Homeserver:  meta.Homeserver,
			UserID:      meta.UserID,
			AccessToken: token,
			DeviceID:    meta.DeviceID,
		})
		if err != nil {
			m.logger.Error("failed to restore session",
				"user", userID,
				"err", err,
			)
			continue
		}

		if credentials.LoadVerified(userID) {
			m.mu.Lock()
			m.verifiedUsers[userID] = true
			m.mu.Unlock()
		}

		if rk, rkErr := credentials.LoadRecoveryKey(userID); rkErr == nil && rk != "" {
			m.mu.Lock()
			m.recoveryKeys[userID] = rk
			m.mu.Unlock()
		}
	}
}

func (m *Manager) RestoreSession(
	sess models.MatrixSession,
) error {
	client, err := mautrix.NewClient(
		sess.Homeserver,
		id.UserID(sess.UserID),
		sess.AccessToken,
	)
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}
	client.DeviceID = id.DeviceID(sess.DeviceID)

	m.mu.Lock()
	m.clients[sess.UserID] = client
	m.mu.Unlock()

	ctx := context.Background()
	dbPath := fmt.Sprintf(
		"%s/%s.db",
		m.cryptoDBPath,
		url.PathEscape(sess.UserID),
	)
	if err := m.setupCrypto(
		ctx, sess.UserID, dbPath, m.pickleKey,
	); err != nil {
		m.logger.Error("crypto setup failed on restore",
			"user", sess.UserID,
			"err", err,
		)
	}

	m.startSync(sess.UserID, client)

	return nil
}

func (m *Manager) Logout(ctx context.Context, userID string) error {
	m.mu.Lock()
	client, ok := m.clients[userID]
	cancel, hasCancel := m.cancels[userID]
	helper, hasHelper := m.cryptoHelpers[userID]
	delete(m.clients, userID)
	delete(m.cancels, userID)
	delete(m.cryptoHelpers, userID)
	delete(m.verificationStates, userID)
	delete(m.sasSessions, userID)
	m.mu.Unlock()

	if hasHelper && helper != nil {
		_ = helper.Close()
	}
	if hasCancel {
		cancel()
	}

	credentials.DeleteSession(userID)
	credentials.DeleteRecoveryKey(userID)
	credentials.DeleteVerified(userID)
	_ = credentials.RemoveKnownUser(userID)

	if ok {
		_, err := client.Logout(ctx)
		return err
	}
	return nil
}

func (m *Manager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for userID, cancel := range m.cancels {
		cancel()
		delete(m.cancels, userID)
	}

	for userID, helper := range m.cryptoHelpers {
		if helper != nil {
			if err := helper.Close(); err != nil {
				m.logger.Error("failed to close crypto helper",
					"user", userID,
					"err", err,
				)
			}
		}
		delete(m.cryptoHelpers, userID)
	}
}

func (m *Manager) SetRecoveryKey(userID string, key string) {
	m.mu.Lock()
	m.recoveryKeys[userID] = key
	m.mu.Unlock()

	if err := credentials.StoreRecoveryKey(userID, key); err != nil {
		m.logger.Warn("failed to store recovery key in keyring",
			"user", userID,
			"err", err,
		)
	}
}

func (m *Manager) GetRecoveryKey(userID string) string {
	m.mu.RLock()
	key := m.recoveryKeys[userID]
	m.mu.RUnlock()

	if key != "" {
		return key
	}

	stored, err := credentials.LoadRecoveryKey(userID)
	if err != nil {
		return ""
	}

	m.mu.Lock()
	m.recoveryKeys[userID] = stored
	m.mu.Unlock()

	return stored
}

func (m *Manager) startSync(userID string, client *mautrix.Client) {
	ctx, cancel := context.WithCancel(context.Background())
	m.mu.Lock()
	if oldCancel, ok := m.cancels[userID]; ok {
		oldCancel()
	}
	m.cancels[userID] = cancel
	m.mu.Unlock()

	syncer := client.Syncer.(*mautrix.DefaultSyncer)

	syncer.OnEventType(
		event.EventMessage,
		func(ctx context.Context, evt *event.Event) {
			if evt.Sender.String() == userID {
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

	m.mu.RLock()
	helper := m.cryptoHelpers[userID]
	m.mu.RUnlock()

	if helper != nil {
		helper.CustomPostDecrypt = func(
			ctx context.Context, evt *event.Event,
		) {
			if evt.Type != event.EventMessage {
				return
			}
			if evt.Sender.String() == userID {
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
	}

	syncer.OnEventType(
		event.ToDeviceVerificationRequest,
		func(ctx context.Context, evt *event.Event) {
			m.handleVerificationRequest(ctx, userID, client, evt)
		},
	)
	syncer.OnEventType(
		event.ToDeviceVerificationStart,
		func(ctx context.Context, evt *event.Event) {
			m.handleVerificationStart(ctx, userID, client, evt)
		},
	)
	syncer.OnEventType(
		event.ToDeviceVerificationKey,
		func(ctx context.Context, evt *event.Event) {
			m.handleVerificationKey(ctx, userID, client, evt)
		},
	)
	syncer.OnEventType(
		event.ToDeviceVerificationMAC,
		func(ctx context.Context, evt *event.Event) {
			m.handleVerificationMAC(ctx, userID, client, evt)
		},
	)
	syncer.OnEventType(
		event.ToDeviceVerificationCancel,
		func(ctx context.Context, evt *event.Event) {
			m.handleVerificationCancel(userID, evt)
		},
	)
	syncer.OnEventType(
		event.ToDeviceVerificationDone,
		func(ctx context.Context, evt *event.Event) {
			m.handleVerificationDone(userID, evt)
		},
	)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				err := client.SyncWithContext(ctx)
				if err != nil && ctx.Err() == nil {
					m.logger.Error("sync error",
						"user", userID,
						"err", err,
					)
					time.Sleep(5 * time.Second)
				}
			}
		}
	}()
}

func (m *Manager) eventToHTML(
	client *mautrix.Client,
	evt *event.Event,
) []byte {
	content, ok := evt.Content.Parsed.(*event.MessageEventContent)
	if !ok {
		return nil
	}

	senderName := evt.Sender.Localpart()
	hsURL := client.HomeserverURL.String()
	avatarURL := fmt.Sprintf(
		"https://api.dicebear.com/7.x/avataaars/svg?seed=%s",
		senderName,
	)

	profile, _ := client.GetProfile(context.Background(), evt.Sender)
	if profile != nil {
		if profile.DisplayName != "" {
			senderName = profile.DisplayName
		}
		avatarURL = resolveContentURI(
			hsURL, profile.AvatarURL, evt.Sender.Localpart(), "avataaars",
		)
	}

	msg := models.Message{
		ID:      evt.ID.String(),
		Content: content.Body,
		Author: models.User{
			ID:     evt.Sender.String(),
			Name:   senderName,
			Avatar: avatarURL,
			Status: models.StatusOnline,
		},
		Timestamp: time.UnixMilli(evt.Timestamp),
		ChannelID: evt.RoomID.String(),
	}

	var inner bytes.Buffer
	if err := ui.MessageBubble(msg).Render(
		context.Background(), &inner,
	); err != nil {
		return nil
	}

	var payload bytes.Buffer
	payload.WriteString(
		`<div id="message-list" hx-swap-oob="beforeend">`,
	)
	payload.Write(inner.Bytes())
	payload.WriteString(`</div>`)

	return payload.Bytes()
}
