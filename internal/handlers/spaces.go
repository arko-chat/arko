package handlers

import (
	"net/http"

	"github.com/arko-chat/arko/internal/htmx"
	spacespage "github.com/arko-chat/arko/pages/spaces"
	"github.com/go-chi/chi/v5"
)

func (h *Handler) HandleSpaces(w http.ResponseWriter, r *http.Request) {
	state := h.session(r)
	ctx := r.Context()
	spaceID := chi.URLParam(r, "spaceID")

	user, err := h.svc.User.GetCurrentUser(ctx)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	spaces, err := h.svc.Spaces.ListSpaces(ctx)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	detail, err := h.svc.Spaces.GetSpace(ctx, spaceID)
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

	if err := spacespage.Page(state, user, spaces, detail).Render(ctx, w); err != nil {
		h.serverError(w, r, err)
	}
}
