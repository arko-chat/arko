package router

import (
	"net/http"

	"github.com/arko-chat/arko/components/assets"
	"github.com/arko-chat/arko/internal/handlers"
	"github.com/arko-chat/arko/internal/matrix"
	"github.com/arko-chat/arko/internal/middleware"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

func New(
	h *handlers.Handler,
	mgr *matrix.Manager,
) *chi.Mux {
	r := chi.NewRouter()

	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.RealIP)
	r.Use(chimw.RequestID)
	r.Use(middleware.SessionMiddleware())

	if dist := assets.DistFS(); dist != nil {
		r.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(dist))))
	}

	r.Get("/login", h.HandleLoginPage)
	r.Post("/login/submit", h.HandleLoginSubmit)
	r.Get("/logout", h.HandleLogout)

	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(mgr, middleware.AuthPages{
			Login:  h.HandleLoginPage,
			Verify: h.HandleVerifyPage,
		}))

		r.Get("/verify", h.HandleVerifyPage)
		r.Post("/verify/submit", h.HandleVerifySubmit)
		r.Get("/verify/waiting", h.HandleVerifyWaitingPage)
		r.Get("/verify/sas", h.HandleVerifySASPage)
		r.Post("/verify/confirm", h.HandleVerifyConfirm)
		r.Post("/verify/cancel", h.HandleVerifyCancel)

		r.Get("/", h.HandleFriends)
		r.Get("/friends", h.HandleFriendsFilter)
		r.Post("/friends/search", h.HandleFriendSearch)
		r.Get("/dm/{userID}", h.HandleDM)
		r.Get("/spaces/{spaceID}", h.HandleSpaces)
		r.Get("/spaces/{spaceID}/channels/{channelID}", h.HandleChannels)
		r.Get("/ws/room/{roomID}", h.HandleRoom)

		r.Get("/api/media", h.HandleProxyMedia)
		r.Post("/api/theme", h.HandleToggleTheme)
	})

	return r
}
