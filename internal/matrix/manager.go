package matrix

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/id"

	"github.com/arko-chat/arko/internal/cache"
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
	ctx            context.Context
	cancel         context.CancelFunc
	logger         *slog.Logger
	cryptoDBPath   string
	sentMsgIds     *lru.Cache[string, struct{}]
	matrixSessions *xsync.Map[string, *MatrixSession]
	currSession    atomic.Pointer[MatrixSession]
	verifiedCache  bool

	userCache     *xsync.Map[string, cache.CacheEntry[models.User]]
	userSfg       *singleflight.Group
	roomCache     *xsync.Map[string, cache.CacheEntry[string]]
	roomNameSfg   *singleflight.Group
	roomAvatarSfg *singleflight.Group
	channelsCache *xsync.Map[string, cache.CacheEntry[[]models.Channel]]
	channelsSfg   *singleflight.Group
	spacesCache   *xsync.Map[string, cache.CacheEntry[[]models.Space]]
	spacesSfg     *singleflight.Group
	dmCache       *xsync.Map[string, cache.CacheEntry[[]models.User]]
	dmSfg         *singleflight.Group
	membersCache  *xsync.Map[string, cache.CacheEntry[[]models.User]]
	membersSfg    *singleflight.Group
}

func NewManager(
	hub *ws.Hub,
	logger *slog.Logger,
	cryptoDBPath string,
) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	newLru, _ := lru.New[string, struct{}](50)
	m := &Manager{
		hub:            hub,
		ctx:            ctx,
		cancel:         cancel,
		logger:         logger,
		cryptoDBPath:   cryptoDBPath,
		sentMsgIds:     newLru,
		matrixSessions: xsync.NewMap[string, *MatrixSession](),
		userCache:      xsync.NewMap[string, cache.CacheEntry[models.User]](),
		roomCache:      xsync.NewMap[string, cache.CacheEntry[string]](),
		channelsCache:  xsync.NewMap[string, cache.CacheEntry[[]models.Channel]](),
		spacesCache:    xsync.NewMap[string, cache.CacheEntry[[]models.Space]](),
		dmCache:        xsync.NewMap[string, cache.CacheEntry[[]models.User]](),
		membersCache:   xsync.NewMap[string, cache.CacheEntry[[]models.User]](),
		userSfg:        &singleflight.Group{},
		roomNameSfg:    &singleflight.Group{},
		roomAvatarSfg:  &singleflight.Group{},
		spacesSfg:      &singleflight.Group{},
		dmSfg:          &singleflight.Group{},
		membersSfg:     &singleflight.Group{},
		channelsSfg:    &singleflight.Group{},
	}

	m.restoreAllSessions()

	return m
}

func (m *Manager) GetContext() context.Context {
	return m.ctx
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

	ctx, cancel := context.WithTimeout(m.ctx, 10*time.Second)
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
	if m.cancel != nil {
		m.cancel()
	}
	mSess, _ := m.matrixSessions.LoadAndDelete(userID)
	mSess.GetClient().Logout(ctx)
	mSess.Close()
	session.Delete(userID)
	return nil
}

func (m *Manager) Shutdown() {
	if m.cancel != nil {
		m.cancel()
	}
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

func (m *Manager) startSync(sess *session.Session, client *mautrix.Client) {
	_, exists := m.matrixSessions.Load(sess.UserID)
	if exists {
		return
	}

	newSession, err := m.NewMatrixSession(m.ctx, client, m.logger)
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
