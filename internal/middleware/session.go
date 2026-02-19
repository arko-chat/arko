package middleware

import (
	"context"
	"net/http"

	"github.com/arko-chat/arko/internal/credentials"
	"github.com/arko-chat/arko/internal/session"
)

type contextKey string

const stateKey = contextKey("state")

func Session(store *session.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			state := store.Load(r)
			ctx := setStateContext(r.Context(), state)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetState(ctx context.Context) session.State {
	if s, ok := ctx.Value(stateKey).(session.State); ok {
		return s
	}
	return session.DefaultState(nil)
}

func setStateContext(ctx context.Context, state session.State) context.Context {
	return context.WithValue(ctx, stateKey, state)
}

func AutoRestore(store *session.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			state := GetState(r.Context())

			if state.LoggedIn && state.UserID != "" {
				next.ServeHTTP(w, r)
				return
			}

			users := credentials.GetKnownUsers()
			if len(users) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			userID := users[0]
			meta, token, err := credentials.LoadSession(userID)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			state.UserID = meta.UserID
			state.Homeserver = meta.Homeserver
			state.AccessToken = token
			state.DeviceID = meta.DeviceID
			state.LoggedIn = true
			state.Verified = credentials.LoadVerified(userID)

			if err := store.SaveState(w, r, state); err != nil {
				next.ServeHTTP(w, r)
				return
			}

			ctx := setStateContext(r.Context(), state)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
