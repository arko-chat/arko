package handlers

import (
	"net/http"

	"github.com/arko-chat/arko/components/pages"
	"github.com/go-chi/chi/v5"
)

func (h *Handler) HandleChannel(w http.ResponseWriter, r *http.Request) {
	state := h.state(r)
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

	roomID := channelID

	if err := pages.ChannelPageContent(user, spaces, detail, ch, messages, roomID).Render(ctx, w); err != nil {
		h.serverError(w, r, err)
	}
	return
}
