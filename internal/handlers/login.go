package handlers

import (
	"net/http"
	"strings"

	"github.com/arko-chat/arko/components/ui"
	"github.com/arko-chat/arko/internal/credentials"
	"github.com/arko-chat/arko/internal/htmx"
	"github.com/arko-chat/arko/internal/models"
	loginpage "github.com/arko-chat/arko/pages/login"
)

func (h *Handler) HandleLoginPage(
	w http.ResponseWriter,
	r *http.Request,
) {
	state := h.state(r)
	if state.LoggedIn && state.UserID != "" {
		htmx.Redirect(w, r, "/")
		return
	}

	if err := loginpage.Page().Render(r.Context(), w); err != nil {
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

	knownUsers := credentials.GetKnownUsers()
	for _, uid := range knownUsers {
		meta, _, err := credentials.LoadSession(uid)
		if err != nil {
			continue
		}
		if meta.DeviceID != "" &&
			strings.Contains(uid, creds.Username) {
			creds.DeviceID = meta.DeviceID
			break
		}
	}

	sess, err := h.svc.Login(r.Context(), creds)
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

	state := h.state(r)
	state.UserID = sess.UserID
	state.Homeserver = sess.Homeserver
	state.AccessToken = sess.AccessToken
	state.DeviceID = sess.DeviceID
	state.LoggedIn = true

	if err := state.Save(w, r); err != nil {
		h.serverError(w, r, err)
		return
	}

	w.Header().Set("HX-Redirect", "/")
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) HandleLogout(
	w http.ResponseWriter,
	r *http.Request,
) {
	state := h.state(r)

	if state.LoggedIn {
		_ = h.svc.Logout(r.Context(), state.UserID)
	}

	state.LoggedIn = false
	state.UserID = ""
	state.AccessToken = ""
	state.DeviceID = ""
	state.Homeserver = ""
	_ = state.Clear(w, r)

	htmx.Redirect(w, r, "/login")
}
