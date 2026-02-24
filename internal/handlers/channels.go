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

	user, err := h.svc.User.GetCurrentUser()
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	spaces, err := h.svc.Spaces.ListSpaces()
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	detail, err := h.svc.Spaces.GetSpace(spaceID)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	ch, err := h.svc.Spaces.GetChannel(spaceID, channelID)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	messages, err := h.svc.Chat.GetRoomMessages(channelID)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	messagesArr := messages.Chronological()

	roomID := ch.ID

	if htmx.IsHTMX(r) {
		if err := channelspage.Content(user, spaces, detail, ch, messagesArr, roomID).Render(ctx, w); err != nil {
			h.serverError(w, r, err)
		}
		return
	}

	if err := channelspage.Page(state, user, spaces, detail, ch, messagesArr, roomID).Render(ctx, w); err != nil {
		h.serverError(w, r, err)
	}
}
