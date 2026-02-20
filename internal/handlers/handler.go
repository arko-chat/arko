package handlers

import (
	"log/slog"
	"net/http"

	"github.com/arko-chat/arko/internal/middleware"
	"github.com/arko-chat/arko/internal/service"
	"github.com/arko-chat/arko/internal/session"
)

type Handler struct {
	svc    *service.ChatService
	logger *slog.Logger
}

func New(svc *service.ChatService, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, logger: logger}
}

func (h *Handler) session(r *http.Request) *session.Session {
	return middleware.GetSession(r.Context())
}

func (h *Handler) serverError(w http.ResponseWriter, r *http.Request, err error) {
	h.logger.Error("server error", "path", r.URL.Path, "err", err)
	http.Error(w, "Internal Server Error", http.StatusInternalServerError)
}
