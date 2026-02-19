package handlers

import (
	"net/http"

	"github.com/arko-chat/arko/components/pages"
	"github.com/go-chi/chi/v5"
)

func (h *Handler) HandleDM(w http.ResponseWriter, r *http.Request) {
	state := h.state(r)
	ctx := r.Context()
	otherID := chi.URLParam(r, "userID")

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

	friendsList, err := h.svc.ListFriends(ctx, state.UserID)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	friend, err := h.svc.GetFriend(ctx, state.UserID, otherID)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	messages, err := h.svc.GetDMMessages(ctx, state.UserID, otherID)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	roomID, err := h.svc.GetDMRoomID(ctx, state.UserID, otherID)
	if err != nil {
		roomID = "dm-" + otherID
	}

	if err := pages.DMPageContent(user, spaces, friendsList, friend, messages, roomID).Render(ctx, w); err != nil {
		h.serverError(w, r, err)
	}
	return
}
