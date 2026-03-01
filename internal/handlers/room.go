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
