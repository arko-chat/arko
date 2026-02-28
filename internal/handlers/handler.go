package handlers

import (
	"bytes"
	"log/slog"
	"net/http"
	"time"

	"github.com/arko-chat/arko/components/ui"
	"github.com/arko-chat/arko/internal/cache"
	"github.com/arko-chat/arko/internal/middleware"
	"github.com/arko-chat/arko/internal/service"
	"github.com/arko-chat/arko/internal/session"
	"github.com/arko-chat/arko/internal/ws"
	"github.com/gorilla/websocket"
)

type Handler struct {
	hub        *ws.Hub
	svc        *service.Services
	logger     *slog.Logger
	mediaCache *cache.Cache[MediaResponse]
}

func New(hub *ws.Hub, svc *service.Services, logger *slog.Logger) *Handler {
	return &Handler{
		hub:        hub,
		svc:        svc,
		logger:     logger,
		mediaCache: cache.New[MediaResponse](24 * time.Hour),
	}
}

func (h *Handler) session(r *http.Request) *session.Session {
	return middleware.GetSession(r.Context())
}

func (h *Handler) serverError(w http.ResponseWriter, r *http.Request, err error) {
	h.logger.Error("server error", "err", err, "path", r.URL.Path)
	w.WriteHeader(http.StatusInternalServerError)
	_ = ui.Alert("Something went wrong. Please try again.").Render(r.Context(), w)
}

func (h *Handler) clientError(w http.ResponseWriter, r *http.Request, status int, message string) {
	w.WriteHeader(status)
	_ = ui.Alert(message).Render(r.Context(), w)
}

func (h *Handler) wsError(conn *websocket.Conn, message string) {
	var buf bytes.Buffer
	_ = ui.ToastError(message).Render(nil, &buf)
	_ = conn.WriteMessage(websocket.TextMessage, buf.Bytes())
}
