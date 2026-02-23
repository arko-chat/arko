package handlers

import (
	"log/slog"
	"net/http"

	"github.com/arko-chat/arko/internal/cache"
	"github.com/arko-chat/arko/internal/middleware"
	"github.com/arko-chat/arko/internal/service"
	"github.com/arko-chat/arko/internal/session"
	"github.com/puzpuzpuz/xsync/v4"
	"golang.org/x/sync/singleflight"
)

type Handler struct {
	svc        *service.Services
	logger     *slog.Logger
	mediaCache *xsync.Map[string, cache.CacheEntry[*MediaResponse]]
	mediaSfg   *singleflight.Group
}

func New(svc *service.Services, logger *slog.Logger) *Handler {
	return &Handler{
		svc:        svc,
		logger:     logger,
		mediaCache: xsync.NewMap[string, cache.CacheEntry[*MediaResponse]](),
		mediaSfg:   &singleflight.Group{},
	}
}

func (h *Handler) session(r *http.Request) *session.Session {
	return middleware.GetSession(r.Context())
}

func (h *Handler) serverError(w http.ResponseWriter, r *http.Request, err error) {
	h.logger.Error("server error", "path", r.URL.Path, "err", err)
	http.Error(w, "Internal Server Error", http.StatusInternalServerError)
}
