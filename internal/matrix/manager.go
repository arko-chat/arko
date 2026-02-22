// TODO: fix messages from other clients not received in real time
// TODO: aggressive caching for a more responsive experience
// TODO: integrate matrix events to messagetree
// TODO: map UI format in real time with messagetree

package matrix

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"github.com/arko-chat/arko/internal/models"
	"github.com/arko-chat/arko/internal/session"
	"github.com/arko-chat/arko/internal/ws"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/puzpuzpuz/xsync/v4"
)

var ErrNoClient = errors.New("no client for user")
var ErrNotVerified = errors.New(
	"device not verified: cross-signing setup required",
)

type Manager struct {
	hub            *ws.Hub
	logger         *slog.Logger
	cryptoDBPath   string
	sentMsgIds     *lru.Cache[string, struct{}]
	matrixSessions *xsync.Map[string, *MatrixSession]
	currSession    atomic.Pointer[MatrixSession]
	verifiedCache  bool
}

func NewManager(
	hub *ws.Hub,
	logger *slog.Logger,
	cryptoDBPath string,
) *Manager {
	newLru, _ := lru.New[string, struct{}](50)

	m := &Manager{
		hub:            hub,
		logger:         logger,
		cryptoDBPath:   cryptoDBPath,
		sentMsgIds:     newLru,
		matrixSessions: xsync.NewMap[string, *MatrixSession](),
	}

	m.restoreAllSessions()

	return m
}

func (m *Manager) HasClient(userID string) bool {
	_, ok := m.matrixSessions.Load(userID)
	return ok
}

func (m *Manager) GetClient(userID string) (*mautrix.Client, error) {
	session, ok := m.matrixSessions.Load(userID)
	if !ok {
		return nil, ErrNoClient
	}
	return session.GetClient(), nil
}

func (m *Manager) GetVerificationState(
	userID string,
) *VerificationUIState {
	session, ok := m.matrixSessions.Load(userID)
	if !ok {
		return nil
	}
	return session.GetVerificationUIState()
}

func (m *Manager) GetSupportedAuthTypes(ctx context.Context, creds models.LoginCredentials) ([]mautrix.AuthType, error) {
	wellknown, err := mautrix.DiscoverClientAPI(ctx, creds.Homeserver)
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}

	client, err := mautrix.NewClient(wellknown.Homeserver.BaseURL, "", "")
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}

	flowsResp, err := client.GetLoginFlows(ctx)
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}

	types := make([]mautrix.AuthType, 0, len(flowsResp.Flows))
	for _, flow := range flowsResp.Flows {
		types = append(types, flow.Type)
	}

	return types, nil
}

func (m *Manager) Login(
	ctx context.Context,
	creds models.LoginCredentials,
) (*session.Session, error) {
	globalConf, err := session.GetGlobalSettings()
	if err != nil {
		return nil, fmt.Errorf("get global settings: %w", err)
	}

	var newSession *session.Session
	userID := ""
	accessToken := ""

	if globalConf.LastUserID != "" {
		userID = globalConf.LastUserID
		newSession, err = session.Get(userID)
		if err == nil {
			accessToken = newSession.AccessToken
		}
	}

	wellknown, err := mautrix.DiscoverClientAPI(ctx, creds.Homeserver)
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}

	client, err := mautrix.NewClient(wellknown.Homeserver.BaseURL, id.UserID(userID), accessToken)
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}

	if userID == "" || accessToken == "" {
		token, err := m.GetSSOToken(ctx, client)
		if err != nil {
			return nil, fmt.Errorf("create client: %w", err)
		}
		m.logger.Debug("got token", "token", token)
		loginReq := &mautrix.ReqLogin{
			Type:  mautrix.AuthTypeToken,
			Token: token,
			Identifier: mautrix.UserIdentifier{
				Type: mautrix.IdentifierTypeUser,
				User: creds.Username,
			},
			InitialDeviceDisplayName: "Arko Desktop Client",
			StoreCredentials:         true,
		}

		if creds.DeviceID != "" {
			loginReq.DeviceID = id.DeviceID(creds.DeviceID)
		}

		resp, err := client.Login(ctx, loginReq)
		if err != nil {
			supported, _ := m.GetSupportedAuthTypes(ctx, creds)
			m.logger.Info("supported auth types", "supported", supported)
			return nil, fmt.Errorf("login: %w", err)
		}

		idServer := ""
		if resp.WellKnown != nil {
			idServer = resp.WellKnown.IdentityServer.BaseURL
		}

		newSession = &session.Session{
			Homeserver:     wellknown.Homeserver.BaseURL,
			Identityserver: idServer,
			UserID:         resp.UserID.String(),
			AccessToken:    resp.AccessToken,
			RefreshToken:   resp.RefreshToken,
			ExpiresInMs:    resp.ExpiresInMS,
			DeviceID:       string(resp.DeviceID),
		}

		newSession, err = session.UpdateAndGet(newSession.UserID, func(s *session.Session) {
			s.Homeserver = newSession.Homeserver
			s.Identityserver = newSession.Identityserver
			s.AccessToken = newSession.AccessToken
			s.RefreshToken = newSession.RefreshToken
			s.ExpiresInMs = newSession.ExpiresInMs
		})
		if err != nil {
			m.logger.Error("failed to store session in keyring",
				"user", newSession.UserID,
				"err", err,
			)
		}
	}

	m.startSync(newSession, client)
	return newSession, nil
}

func (m *Manager) restoreAllSessions() {
	users := session.GetKnownUsers()
	for _, userID := range users {
		m.logger.Info("restoring session",
			"user", userID,
		)

		session, err := session.Get(userID)
		if err != nil {
			m.logger.Warn("skipping stored session",
				"user", userID,
				"err", err,
			)
			continue
		}

		err = m.restoreSession(session)
		if err != nil {
			m.logger.Error("failed to restore session",
				"user", userID,
				"err", err,
			)
			continue
		}
	}
}

func (m *Manager) restoreSession(
	sess *session.Session,
) error {
	client, err := mautrix.NewClient(
		sess.Homeserver,
		id.UserID(sess.UserID),
		sess.AccessToken,
	)
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	whoami, err := client.Whoami(ctx)
	if err != nil {
		if sess.RefreshToken != "" {
			resp, refreshErr := m.doRefreshToken(ctx, client, sess.RefreshToken)
			if refreshErr != nil {
				session.Delete(sess.UserID)
				return fmt.Errorf("token expired and refresh failed: %w", refreshErr)
			}
			client.AccessToken = resp.AccessToken
			newSess, _ := session.UpdateAndGet(sess.UserID, func(s *session.Session) {
				s.AccessToken = resp.AccessToken
				if resp.RefreshToken != "" {
					s.RefreshToken = resp.RefreshToken
				}
				if resp.ExpiresInMs > 0 {
					s.ExpiresInMs = resp.ExpiresInMs
				}
			})
			if newSess != nil {
				sess = newSess
			}
		} else {
			session.Delete(sess.UserID)
			return fmt.Errorf("token invalid and no refresh token: %w", err)
		}
	} else if whoami.DeviceID != "" {
		client.DeviceID = whoami.DeviceID
	}

	m.startSync(sess, client)
	return nil
}

func (m *Manager) Logout(ctx context.Context, userID string) error {
	mSess, _ := m.matrixSessions.LoadAndDelete(userID)
	mSess.GetClient().Logout(ctx)
	mSess.Close()
	session.Delete(userID)
	return nil
}

func (m *Manager) Shutdown() {
	for _, sess := range m.matrixSessions.All() {
		sess.Close()
	}
	m.matrixSessions.Clear()
}

func (m *Manager) GetCurrentUserID() string {
	currSess := m.currSession.Load()
	return currSess.id
}

func (m *Manager) GetCurrentMatrixSession() *MatrixSession {
	currSess := m.currSession.Load()
	return currSess
}

func (m *Manager) GetMatrixSession(userId string) *MatrixSession {
	sess, _ := m.matrixSessions.Load(userId)
	return sess
}

func (m *Manager) SetRecoveryKey(userID string, key string) {
	if err := session.Update(userID, func(s *session.Session) {
		s.RecoveryKey = key
	}); err != nil {
		m.logger.Warn("failed to store recovery key in keyring",
			"user", userID,
			"err", err,
		)
	}
}

func (m *Manager) GetRecoveryKey(userID string) string {
	s, err := session.Get(userID)
	if err != nil {
		return ""
	}

	return s.RecoveryKey
}

func (m *Manager) IsVerified(ctx context.Context, userID string) bool {
	session, ok := m.matrixSessions.Load(userID)
	if !ok {
		return false
	}

	machine := session.GetCryptoHelper().Machine()
	if machine == nil {
		return false
	}

	device, err := machine.CryptoStore.GetDevice(
		ctx, id.UserID(userID), machine.Client.DeviceID,
	)
	if err != nil || device == nil {
		m.logger.Error("failed to get device",
			"user", userID,
			"error", err,
		)
		return false
	}

	trust, err := machine.ResolveTrustContext(ctx, device)
	if err != nil {
		m.logger.Error("failed to resolve trust",
			"user", userID,
			"error", err,
		)
		return false
	}

	if trust == id.TrustStateCrossSignedTOFU {
		return true
	}

	m.logger.Debug("device not verified",
		"user", userID,
		"trust", trust.String(),
	)
	return false
}

func (m *Manager) startSync(sess *session.Session, client *mautrix.Client) {
	_, exists := m.matrixSessions.Load(sess.UserID)
	if exists {
		return
	}

	newSession, err := m.NewMatrixSession(client)
	if err != nil {
		m.logger.Error("sync error",
			"user", sess.UserID,
			"err", err,
		)
		return
	}

	m.matrixSessions.Store(sess.UserID, newSession)
	m.currSession.Store(newSession)
}

func (m *Manager) EventToMessage(
	client *mautrix.Client,
	evt *event.Event,
) *models.Message {
	content, ok := evt.Content.Parsed.(*event.MessageEventContent)
	if !ok {
		return nil
	}

	senderName := evt.Sender.Localpart()
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
			profile.AvatarURL, evt.Sender.Localpart(), "avataaars",
		)
	}

	m.logger.Debug("event to message",
		"eventID", evt.ID.String(),
		"transactionID", evt.Unsigned.TransactionID,
		"nonce", evt.Unsigned.TransactionID != "",
	)

	return &models.Message{
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
		Nonce:     evt.Unsigned.TransactionID,
	}
}
