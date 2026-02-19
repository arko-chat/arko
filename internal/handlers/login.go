package handlers

import (
	"net/http"

	"github.com/arko-chat/arko/components/pages"
	"github.com/arko-chat/arko/internal/models"
)

func (h *Handler) HandleLoginPage(
	w http.ResponseWriter,
	r *http.Request,
) {
	state := h.state(r)
	if state.LoggedIn && state.UserID != "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	if err := pages.LoginPage().Render(r.Context(), w); err != nil {
		h.serverError(w, r, err)
	}
}

func (h *Handler) HandleLoginSubmit(
	w http.ResponseWriter,
	r *http.Request,
) {
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = pages.LoginError("Invalid form data.").Render(r.Context(), w)
		return
	}

	creds := models.LoginCredentials{
		Homeserver: r.FormValue("homeserver"),
		Username:   r.FormValue("username"),
		Password:   r.FormValue("password"),
	}

	if creds.Homeserver == "" || creds.Username == "" || creds.Password == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = pages.LoginError("All fields are required.").Render(r.Context(), w)
		return
	}

	sess, err := h.svc.Login(r.Context(), creds)
	if err != nil {
		h.logger.Error("login failed",
			"homeserver", creds.Homeserver,
			"username", creds.Username,
			"err", err,
		)
		w.WriteHeader(http.StatusUnauthorized)
		_ = pages.LoginError(
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

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
