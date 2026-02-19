package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/arko-chat/arko/internal/matrix"
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

func (h *Handler) state(r *http.Request) session.State {
	return middleware.GetState(r.Context())
}

func (h *Handler) serverError(
	w http.ResponseWriter,
	r *http.Request,
	err error,
) {
	if errors.Is(err, matrix.ErrNoClient) {
		h.logger.Warn("no matrix client, forcing logout", "err", err)
		http.Redirect(w, r, "/logout", http.StatusSeeOther)
		return
	}
	h.logger.Error("handler error", "err", err)
	http.Error(w, "Internal Server Error", http.StatusInternalServerError)
}

