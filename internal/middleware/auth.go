package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/arko-chat/arko/internal/matrix"
	"github.com/arko-chat/arko/internal/models"
)

func Auth(mgr *matrix.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			state := GetState(r.Context())

			if !state.LoggedIn || state.UserID == "" {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			if !mgr.HasClient(state.UserID) {
				if state.AccessToken == "" || state.Homeserver == "" {
					state.LoggedIn = false
					state.UserID = ""
					_ = state.Clear(w, r)
					http.Redirect(w, r, "/login", http.StatusSeeOther)
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
					http.Redirect(w, r, "/login", http.StatusSeeOther)
					return
				}

				if state.Verified {
					mgr.MarkVerified(state.UserID)
				}
			}

			if !state.Verified && !mgr.IsVerified(state.UserID) {
				if !strings.HasPrefix(r.URL.Path, "/verify") {
					http.Redirect(
						w, r, "/verify", http.StatusSeeOther,
					)
					return
				}
			}

			ctx := context.WithValue(r.Context(), stateKey, state)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
