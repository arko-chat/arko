package handlers

import (
	"net/http"

	"github.com/arko-chat/arko/internal/htmx"
	dmpage "github.com/arko-chat/arko/pages/dm"
	"github.com/go-chi/chi/v5"
)

func (h *Handler) HandleDM(w http.ResponseWriter, r *http.Request) {
	state := h.session(r)
	ctx := r.Context()
	otherID := chi.URLParam(r, "userID")

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

	friendsList, _ := h.svc.Friends.ListFriends(ctx)

	friend, err := h.svc.Friends.GetFriend(ctx, otherID)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	roomID, err := h.svc.Friends.GetFriendRoomID(ctx, otherID)
	if err != nil {
		roomID = "dm-" + otherID
	}

	messages, err := h.svc.Chat.GetRoomMessages(ctx, roomID)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	messagesArr := messages.Chronological()

	if htmx.IsHTMX(r) {
		if err := dmpage.Content(user, spaces, friendsList, friend, messagesArr, roomID).Render(ctx, w); err != nil {
			h.serverError(w, r, err)
		}
		return
	}

	if err := dmpage.Page(state, user, spaces, friendsList, friend, messagesArr, roomID).Render(ctx, w); err != nil {
		h.serverError(w, r, err)
	}
}
