package session

import (
	"net/http"

	"github.com/gorilla/sessions"
)

const cookieName = "arko_session"

type Store struct {
	inner *sessions.CookieStore
}

func NewStore(secret []byte) *Store {
	cs := sessions.NewCookieStore(secret)
	cs.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	return &Store{inner: cs}
}

func (s *Store) Load(r *http.Request) State {
	sess, err := s.inner.Get(r, cookieName)
	if err != nil {
		return DefaultState(sess)
	}

	state := DefaultState(sess)

	if v, ok := sess.Values["user_id"].(string); ok {
		state.UserID = v
	}
	if v, ok := sess.Values["homeserver"].(string); ok {
		state.Homeserver = v
	}
	if v, ok := sess.Values["access_token"].(string); ok {
		state.AccessToken = v
	}
	if v, ok := sess.Values["device_id"].(string); ok {
		state.DeviceID = v
	}
	if v, ok := sess.Values["theme"].(string); ok {
		state.Theme = v
	}
	if v, ok := sess.Values["sidebar_open"].(bool); ok {
		state.SidebarOpen = v
	}
	if v, ok := sess.Values["logged_in"].(bool); ok {
		state.LoggedIn = v
	}
	if v, ok := sess.Values["verified"].(bool); ok {
		state.Verified = v
	}

	return state
}

func (s *Store) SaveState(
	w http.ResponseWriter,
	r *http.Request,
	state State,
) error {
	sess, err := s.inner.Get(r, cookieName)
	if err != nil {
		sess, _ = s.inner.New(r, cookieName)
	}
	state.sess = sess
	return state.Save(w, r)
}
