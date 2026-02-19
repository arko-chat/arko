package middleware

import (
	"context"
	"net/http"

	"github.com/arko-chat/arko/internal/session"
)

type contextKey string

const stateKey = contextKey("state")

func Session(store *session.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			state := store.Load(r)
			ctx := context.WithValue(r.Context(), stateKey, state)
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
