package handlers

import (
	"net/http"

	"github.com/arko-chat/arko/components"
	"github.com/arko-chat/arko/internal/htmx"
	dmpage "github.com/arko-chat/arko/pages/dm"
	"github.com/go-chi/chi/v5"
)

func (h *Handler) HandleDM(w http.ResponseWriter, r *http.Request) {
	state := h.session(r)
	ctx := r.Context()
	otherID := chi.URLParam(r, "userID")

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

	friendsList, _ := h.svc.Friends.ListFriends()

	friend, err := h.svc.Friends.GetFriend(otherID)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	roomID, err := h.svc.Friends.GetFriendRoomID(otherID)
	if err != nil {
		roomID = "dm-" + otherID
	}

	tree, err := h.svc.Chat.GetRoomMessageTree(roomID)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	props := dmpage.ContentProps{
		User:        user,
		Spaces:      spaces,
		FriendsList: friendsList,
		Friend:      friend,
		Tree:        tree,
		RoomID:      roomID,
	}

	h.svc.WebView.SetTitle(friend.Name)

	if htmx.IsHTMX(r) {
		if err := dmpage.Content(props).Render(ctx, w); err != nil {
			h.serverError(w, r, err)
		}
		return
	}

	if err := dmpage.Page(dmpage.PageProps{
		PageProps: components.PageProps{
			State: state,
			Title: h.svc.WebView.GetTitle(),
		},
		ContentProps: props,
	}).Render(ctx, w); err != nil {
		h.serverError(w, r, err)
	}
}
