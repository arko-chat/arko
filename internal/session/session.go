package session

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"sync"

	"github.com/gorilla/securecookie"
	"github.com/zalando/go-keyring"
)

const (
	serviceName       = "arko"
	knownUsersKey     = "app:known_users"
	globalSettingsKey = "app:global_settings"
	cookieName        = "arko_uid"
)

var (
	ErrNotFound = errors.New("session: not found")

	mu            sync.RWMutex
	cache         = make(map[string]*Session)
	globalCache   *GlobalSettings
	globalCacheMu sync.RWMutex
	secureCookie  *securecookie.SecureCookie
)

type Session struct {
	UserID         string `json:"user_id"`
	Homeserver     string `json:"homeserver"`
	Identityserver string `json:"identityserver"`
	AccessToken    string `json:"access_token"`
	RefreshToken   string `json:"refresh_token"`
	PickleKey      []byte `json:"pickle_key"`
	RecoveryKey    string `json:"recovery_key,omitempty"`
	Theme          string `json:"theme"`
	SidebarOpen    bool   `json:"sidebar_open"`
	LoggedIn       bool   `json:"logged_in"`
	DeviceID       string `json:"device_id"`
	ExpiresInMs    int64  `json:"expires_ms"`
}

type GlobalSettings struct {
	LastUserID string `json:"last_user_id,omitempty"`
}

func init() {
	cookieKey := make([]byte, 32)
	_, _ = rand.Read(cookieKey)
	secureCookie = securecookie.New(cookieKey, nil)
}

func Default() *Session {
	return &Session{
		Theme:       "dark",
		SidebarOpen: true,
	}
}

func DefaultGlobalSettings() *GlobalSettings {
	return &GlobalSettings{}
}

func save(sess *Session) error {
	data, err := json.Marshal(sess)
	if err != nil {
		return err
	}
	if err := keyring.Set(serviceName, sess.UserID, string(data)); err != nil {
		return err
	}
	mu.Lock()
	cache[sess.UserID] = sess
	mu.Unlock()

	addKnownUser(sess.UserID)
	UpdateGlobalSettings(func(gs *GlobalSettings) {
		gs.LastUserID = sess.UserID
	})
	return nil
}

func Get(userID string) (sess *Session, err error) {
	mu.RLock()
	if sess, ok := cache[userID]; ok {
		mu.RUnlock()
		return sess, nil
	}
	mu.RUnlock()

	raw, err := keyring.Get(serviceName, userID)
	if err != nil {
		return nil, ErrNotFound
	}
	if err := json.Unmarshal([]byte(raw), &sess); err != nil {
		return nil, err
	}

	mu.Lock()
	cache[userID] = sess
	mu.Unlock()

	return sess, nil
}

func Put(sess *Session) error {
	return save(sess)
}

func Update(userID string, fn func(*Session)) error {
	sess, err := Get(userID)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			return err
		}
		sess = &Session{UserID: userID}
	}
	fn(sess)
	return save(sess)
}

func UpdateAndGet(userID string, fn func(*Session)) (*Session, error) {
	sess, err := Get(userID)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			return nil, err
		}
		sess = &Session{UserID: userID}
	}
	fn(sess)
	if err := save(sess); err != nil {
		return nil, err
	}

	return sess, nil
}

func Delete(userID string) {
	_ = keyring.Delete(serviceName, userID)
	mu.Lock()
	delete(cache, userID)
	mu.Unlock()
	removeKnownUser(userID)
}

func GetGlobalSettings() (*GlobalSettings, error) {
	globalCacheMu.RLock()
	if globalCache != nil {
		gs := globalCache
		globalCacheMu.RUnlock()
		return gs, nil
	}
	globalCacheMu.RUnlock()

	raw, err := keyring.Get(serviceName, globalSettingsKey)
	if err != nil {
		return DefaultGlobalSettings(), nil
	}
	var gs GlobalSettings
	if err := json.Unmarshal([]byte(raw), &gs); err != nil {
		return nil, err
	}

	globalCacheMu.Lock()
	globalCache = &gs
	globalCacheMu.Unlock()

	return &gs, nil
}

func PutGlobalSettings(gs *GlobalSettings) error {
	data, err := json.Marshal(gs)
	if err != nil {
		return err
	}
	if err := keyring.Set(serviceName, globalSettingsKey, string(data)); err != nil {
		return err
	}

	globalCacheMu.Lock()
	globalCache = gs
	globalCacheMu.Unlock()

	return nil
}

func UpdateGlobalSettings(fn func(*GlobalSettings)) error {
	gs, err := GetGlobalSettings()
	if err != nil {
		return err
	}
	fn(gs)
	return PutGlobalSettings(gs)
}

func addKnownUser(userID string) error {
	users := GetKnownUsers()
	if slices.Contains(users, userID) {
		return nil
	}
	users = append(users, userID)
	data, _ := json.Marshal(users)
	return keyring.Set(serviceName, knownUsersKey, string(data))
}

func removeKnownUser(userID string) error {
	users := GetKnownUsers()
	filtered := make([]string, 0, len(users))
	for _, u := range users {
		if u != userID {
			filtered = append(filtered, u)
		}
	}
	data, _ := json.Marshal(filtered)
	return keyring.Set(serviceName, knownUsersKey, string(data))
}

func GetKnownUsers() []string {
	raw, err := keyring.Get(serviceName, knownUsersKey)
	if err != nil {
		return nil
	}
	var users []string
	_ = json.Unmarshal([]byte(raw), &users)
	return users
}

func SetCookie(w http.ResponseWriter, session *Session) {
	encoded, _ := secureCookie.Encode(cookieName, session.UserID)
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    encoded,
		Path:     "/",
		MaxAge:   86400 * 30,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func ReadCookie(r *http.Request) string {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return ""
	}

	userID := ""
	secureCookie.Decode(cookieName, c.Value, &userID)

	return userID
}
