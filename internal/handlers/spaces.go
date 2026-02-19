package handlers

import (
	"net/http"

	"github.com/arko-chat/arko/internal/htmx"
	spacespage "github.com/arko-chat/arko/pages/spaces"
	"github.com/go-chi/chi/v5"
)

func (h *Handler) HandleSpaces(w http.ResponseWriter, r *http.Request) {
	state := h.state(r)
	ctx := r.Context()
	spaceID := chi.URLParam(r, "spaceID")

	user, err := h.svc.GetCurrentUser(ctx, state.UserID)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	spaces, err := h.svc.ListSpaces(ctx, state.UserID)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	detail, err := h.svc.GetSpaceDetail(ctx, state.UserID, spaceID)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	if htmx.IsHTMX(r) {
		if err := spacespage.Content(user, spaces, detail).Render(ctx, w); err != nil {
			h.serverError(w, r, err)
		}
		return
	}

	if err := spacespage.Page(user, spaces, detail).Render(ctx, w); err != nil {
		h.serverError(w, r, err)
	}
}
