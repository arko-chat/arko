package handlers

import (
	"net/http"

	"github.com/arko-chat/arko/components"
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

	h.svc.WebView.SetTitle("Login")

	props := loginpage.PageProps{
		PageProps: components.PageProps{
			State: sess,
			Title: h.svc.WebView.GetTitle(),
		},
	}

	if err := loginpage.Page(props).Render(r.Context(), w); err != nil {
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
	}

	if creds.Homeserver == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = ui.Alert("Homeserver field is required.").Render(r.Context(), w)
		return
	}

	result, err := h.svc.User.Login(r.Context(), creds)
	if err != nil {
		h.logger.Error("login failed",
			"homeserver", creds.Homeserver,
			"err", err,
		)
		w.WriteHeader(http.StatusUnauthorized)
		_ = ui.Alert(
			"Login failed. Check your homeserver, username, and password.",
		).Render(r.Context(), w)
		return
	}

	result, err = session.UpdateAndGet(result.UserID, func(s *session.Session) {
		s.UserID = result.UserID
		s.Homeserver = result.Homeserver
		s.AccessToken = result.AccessToken
		s.LoggedIn = true
	})
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	session.SetCookie(w, result)

	w.Header().Set("HX-Redirect", "/")
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) HandleLogout(
	w http.ResponseWriter,
	r *http.Request,
) {
	sess := h.session(r)

	if sess.LoggedIn {
		_ = h.svc.User.Logout(r.Context())
	}

	session.Delete(sess.UserID)
	session.ClearCookie(w)

	htmx.Redirect(w, r, "/login")
}
