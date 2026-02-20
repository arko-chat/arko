package handlers

import (
	"net/http"
	"strings"

	"github.com/arko-chat/arko/components/ui"
	"github.com/arko-chat/arko/internal/htmx"
	"github.com/arko-chat/arko/internal/models"
	"github.com/arko-chat/arko/internal/session"
	loginpage "github.com/arko-chat/arko/pages/login"
)

func (h *Handler) HandleLoginPage(
	w http.ResponseWriter,
	r *http.Request,
) {
	sess := h.session(r)
	if sess.LoggedIn && sess.UserID != "" {
		htmx.Redirect(w, r, "/")
		return
	}

	if err := loginpage.Page(sess).Render(r.Context(), w); err != nil {
		h.serverError(w, r, err)
	}
}

func (h *Handler) HandleLoginSubmit(
	w http.ResponseWriter,
	r *http.Request,
) {
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = ui.Alert("Invalid form data.").Render(r.Context(), w)
		return
	}

	creds := models.LoginCredentials{
		Homeserver: r.FormValue("homeserver"),
		Username:   r.FormValue("username"),
		Password:   r.FormValue("password"),
	}

	if creds.Homeserver == "" || creds.Username == "" || creds.Password == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = ui.Alert("All fields are required.").Render(r.Context(), w)
		return
	}

	for _, uid := range session.GetKnownUsers() {
		saved, err := session.Get(uid)
		if err != nil {
			continue
		}
		if saved.DeviceID != "" &&
			strings.Contains(uid, creds.Username) {
			creds.DeviceID = saved.DeviceID
			break
		}
	}

	result, err := h.svc.Login(r.Context(), creds)
	if err != nil {
		h.logger.Error("login failed",
			"homeserver", creds.Homeserver,
			"username", creds.Username,
			"err", err,
		)
		w.WriteHeader(http.StatusUnauthorized)
		_ = ui.Alert(
			"Login failed. Check your homeserver, username, and password.",
		).Render(r.Context(), w)
		return
	}

	if err := session.Update(result.UserID, func(s *session.Session) {
		s.UserID = result.UserID
		s.Homeserver = result.Homeserver
		s.AccessToken = result.AccessToken
		s.DeviceID = result.DeviceID
		s.LoggedIn = true
	}); err != nil {
		h.serverError(w, r, err)
		return
	}

	session.SetCookie(w, *result)

	w.Header().Set("HX-Redirect", "/")
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) HandleLogout(
	w http.ResponseWriter,
	r *http.Request,
) {
	sess := h.session(r)

	if sess.LoggedIn {
		_ = h.svc.Logout(r.Context(), sess.UserID)
	}

	session.Delete(sess.UserID)
	session.ClearCookie(w)

	htmx.Redirect(w, r, "/login")
}
