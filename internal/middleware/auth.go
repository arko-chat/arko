package middleware

import (
	"net/http"
	"strings"

	"github.com/arko-chat/arko/internal/htmx"
	"github.com/arko-chat/arko/internal/matrix"
	"github.com/arko-chat/arko/internal/session"
)

type AuthPages struct {
	Login  http.HandlerFunc
	Verify http.HandlerFunc
}

func Auth(mgr *matrix.Manager, pages AuthPages) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			isWS := strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
			sess := GetSession(r.Context())

			if isWS {
				next.ServeHTTP(w, r)
				return
			}

			if !sess.LoggedIn || sess.UserID == "" {
				authRedirect(w, r, "/login", pages.Login)
				return
			}

			if !mgr.HasClient(sess.UserID) {
				sess.LoggedIn = false
				session.Delete(sess.UserID)
				session.ClearCookie(w)
				authRedirect(w, r, "/login", pages.Login)
				return
			}

			if !mgr.GetMatrixSession(sess.UserID).IsVerified() {
				if !strings.HasPrefix(r.URL.Path, "/verify") {
					authRedirect(w, r, "/verify", pages.Verify)
					return
				}
			}

			next.ServeHTTP(w, r)
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
