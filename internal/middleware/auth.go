package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/arko-chat/arko/internal/htmx"
	"github.com/arko-chat/arko/internal/matrix"
	"github.com/arko-chat/arko/internal/models"
)

type AuthPages struct {
	Login  http.HandlerFunc
	Verify http.HandlerFunc
}

func Auth(mgr *matrix.Manager, pages AuthPages) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			state := GetState(r.Context())

			if !state.LoggedIn || state.UserID == "" {
				authRedirect(w, r, "/login", pages.Login)
				return
			}

			if !mgr.HasClient(state.UserID) {
				if state.AccessToken == "" || state.Homeserver == "" {
					state.LoggedIn = false
					state.UserID = ""
					_ = state.Clear(w, r)
					authRedirect(w, r, "/login", pages.Login)
					return
				}

				err := mgr.RestoreSession(models.MatrixSession{
					Homeserver:  state.Homeserver,
					UserID:      state.UserID,
					AccessToken: state.AccessToken,
					DeviceID:    state.DeviceID,
				})
				if err != nil {
					state.LoggedIn = false
					state.UserID = ""
					state.AccessToken = ""
					state.DeviceID = ""
					state.Homeserver = ""
					_ = state.Clear(w, r)
					authRedirect(w, r, "/login", pages.Login)
					return
				}

				if state.Verified {
					mgr.MarkVerified(state.UserID)
				}
			}

			if !state.Verified && !mgr.IsVerified(state.UserID) {
				if !strings.HasPrefix(r.URL.Path, "/verify") {
					authRedirect(w, r, "/verify", pages.Verify)
					return
				}
			}

			ctx := context.WithValue(r.Context(), stateKey, state)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func authRedirect(w http.ResponseWriter, r *http.Request, url string, page http.HandlerFunc) {
	if htmx.IsHTMX(r) {
		htmx.Redirect(w, r, url)
		return
	}
	page(w, r)
}
