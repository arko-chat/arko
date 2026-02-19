package session

import (
	"net/http"

	"github.com/gorilla/sessions"
)

type State struct {
	UserID      string
	Homeserver  string
	AccessToken string
	DeviceID    string
	Theme       string
	SidebarOpen bool
	LoggedIn    bool
	Verified    bool

	sess *sessions.Session
}

func DefaultState(sess *sessions.Session) State {
	return State{
		Theme:       "dark",
		SidebarOpen: true,
		sess:        sess,
	}
}

func (s State) Save(w http.ResponseWriter, r *http.Request) error {
	s.sess.Values["user_id"] = s.UserID
	s.sess.Values["homeserver"] = s.Homeserver
	s.sess.Values["access_token"] = s.AccessToken
	s.sess.Values["device_id"] = s.DeviceID
	s.sess.Values["theme"] = s.Theme
	s.sess.Values["sidebar_open"] = s.SidebarOpen
	s.sess.Values["logged_in"] = s.LoggedIn
	s.sess.Values["verified"] = s.Verified
	return s.sess.Save(r, w)
}

func (s State) Clear(w http.ResponseWriter, r *http.Request) error {
	s.sess.Options.MaxAge = -1
	return s.sess.Save(r, w)
}
