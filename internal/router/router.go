package router

import (
	"net/http"

	"github.com/arko-chat/arko/internal/handlers"
	"github.com/arko-chat/arko/internal/matrix"
	"github.com/arko-chat/arko/internal/middleware"
	"github.com/arko-chat/arko/internal/session"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

func New(
	h *handlers.Handler,
	staticDir string,
	sessionStore *session.Store,
	mgr *matrix.Manager,
) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.RealIP)
	r.Use(chimw.RequestID)
	r.Use(middleware.Session(sessionStore))

	r.Handle(
		"/static/*",
		http.StripPrefix(
			"/static/",
			http.FileServer(http.Dir(staticDir)),
		),
	)

	r.Get("/login", h.HandleLoginPage)
	r.Post("/login/submit", h.HandleLoginSubmit)
	r.Get("/logout", h.HandleLogout)

	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(mgr))

		r.Get("/verify", h.HandleVerifyPage)
		r.Post("/verify/submit", h.HandleVerifySubmit)
		r.Post("/verify/confirm", h.HandleVerifyConfirm)
		r.Post("/verify/cancel", h.HandleVerifyCancel)

		r.Get("/", h.HandleFriends)
		r.Get("/friends", h.HandleFriendsFilter)
		r.Post("/friends/search", h.HandleFriendSearch)
		r.Get("/dm/{userID}", h.HandleDM)
		r.Get("/space/{spaceID}", h.HandleSpace)
		r.Get("/space/{spaceID}/channel/{channelID}", h.HandleChannel)
		r.Get("/ws/room/{roomID}", h.HandleRoomWS)
	})

	return r
}
