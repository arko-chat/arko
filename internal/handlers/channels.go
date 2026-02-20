package handlers

import (
	"net/http"

	"github.com/arko-chat/arko/internal/htmx"
	channelspage "github.com/arko-chat/arko/pages/spaces/channels"
	"github.com/go-chi/chi/v5"
)

func (h *Handler) HandleChannels(w http.ResponseWriter, r *http.Request) {
	state := h.session(r)
	ctx := r.Context()
	spaceID := chi.URLParam(r, "spaceID")
	channelID := chi.URLParam(r, "channelID")

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

	ch, err := h.svc.GetChannel(ctx, state.UserID, spaceID, channelID)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	messages, err := h.svc.GetChannelMessages(ctx, state.UserID, channelID)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	roomID := ch.ID

	if htmx.IsHTMX(r) {
		if err := channelspage.Content(user, spaces, detail, ch, messages, roomID).Render(ctx, w); err != nil {
			h.serverError(w, r, err)
		}
		return
	}

	if err := channelspage.Page(state, user, spaces, detail, ch, messages, roomID).Render(ctx, w); err != nil {
		h.serverError(w, r, err)
	}
}
