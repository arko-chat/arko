package middleware

import (
	"context"
	"net/http"

	"github.com/arko-chat/arko/internal/session"
)

type contextKey string

const stateKey = contextKey("session")

func SessionMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := session.ReadCookie(r)
			var sess session.Session

			if userID != "" {
				loaded, err := session.Get(userID)
				if err == nil {
					sess = loaded
				}
			}

			if !sess.LoggedIn && userID == "" {
				users := session.GetKnownUsers()
				if len(users) > 0 {
					loaded, err := session.Get(users[0])
					if err == nil && loaded.LoggedIn {
						sess = loaded
						session.SetCookie(w, sess)
					}
				}
			}

			if sess.UserID == "" {
				sess = session.Default()
			}

			ctx := context.WithValue(r.Context(), stateKey, &sess)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetSession(ctx context.Context) *session.Session {
	if s, ok := ctx.Value(stateKey).(*session.Session); ok {
		return s
	}
	def := session.Default()
	return &def
}
