package handlers

import (
	"net/http"

	"github.com/arko-chat/arko/components/ui"
	"github.com/go-chi/chi/v5"
)

func (h *Handler) HandleNextMessages(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	roomID := chi.URLParam(r, "roomID")

	hasMore, err := h.svc.Chat.LoadNextMessages(roomID, 30)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	if !hasMore {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if err := ui.MoreMessageScrollSensor(roomID).Render(ctx, w); err != nil {
		h.serverError(w, r, err)
	}
}

func (h *Handler) HandleTyping(w http.ResponseWriter, r *http.Request) {
	s := h.session(r)
	if s == nil || !s.LoggedIn {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	roomID := r.FormValue("roomID")
	if roomID == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := h.svc.Chat.SendTyping(roomID, s.UserID, true); err != nil {
		h.logger.Warn("failed to send typing notification", "err", err)
	}

	w.WriteHeader(http.StatusNoContent)
}
