package handlers

import (
	"fmt"
	"net/http"

	"github.com/arko-chat/arko/components"
	"github.com/arko-chat/arko/components/features/friends"
	"github.com/arko-chat/arko/components/layout/sidebar"
	friendmodal "github.com/arko-chat/arko/components/modals/friends"
	"github.com/arko-chat/arko/internal/htmx"
	dmpage "github.com/arko-chat/arko/pages/dm"
	friendspage "github.com/arko-chat/arko/pages/friends"
)

func (h *Handler) HandleFriends(w http.ResponseWriter, r *http.Request) {
	state := h.session(r)
	ctx := r.Context()

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

	fl, _ := h.svc.Friends.ListFriends()

	props := friendspage.ContentProps{
		User:    user,
		Spaces:  spaces,
		Friends: fl,
	}

	h.svc.WebView.SetTitle("Friends/Direct Messages")

	if htmx.IsHTMX(r) {
		if err := friendspage.Content(props).Render(ctx, w); err != nil {
			h.serverError(w, r, err)
		}
		return
	}

	if err := friendspage.Page(friendspage.PageProps{
		PageProps: components.PageProps{
			State: state,
			Title: h.svc.WebView.GetTitle(),
		},
		ContentProps: props,
	}).Render(ctx, w); err != nil {
		h.serverError(w, r, err)
	}
}

func (h *Handler) HandleFriendsFilter(
	w http.ResponseWriter,
	r *http.Request,
) {
	filter := r.URL.Query().Get("filter")
	if filter == "" {
		filter = "online"
	}

	_, err := h.svc.Friends.FilterFriends(filter)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	if err := friends.View(filter).Render(r.Context(), w); err != nil {
		h.serverError(w, r, err)
	}
}

func (h *Handler) HandleFriendSearch(
	w http.ResponseWriter,
	r *http.Request,
) {
	query := r.FormValue("search")

	filtered, err := h.svc.Friends.SearchFriends(query)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	if err := sidebar.FriendsList(filtered).Render(r.Context(), w); err != nil {
		h.serverError(w, r, err)
	}
}

func (h *Handler) HandleAddFriendModal(w http.ResponseWriter, r *http.Request) {
	if err := friendmodal.AddFriendModal().Render(r.Context(), w); err != nil {
		h.serverError(w, r, err)
	}
}

func (h *Handler) HandleSearchUsers(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")

	if query == "" {
		if err := friendmodal.SearchResults(nil, "").Render(r.Context(), w); err != nil {
			h.serverError(w, r, err)
		}
		return
	}

	users, err := h.svc.Friends.SearchUsers(query)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	if err := friendmodal.SearchResults(users, query).Render(r.Context(), w); err != nil {
		h.serverError(w, r, err)
	}
}

func (h *Handler) HandleCreateDM(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("userID")
	if userID == "" {
		h.serverError(w, r, fmt.Errorf("missing userID"))
		return
	}

	friend, roomID, err := h.svc.Friends.CreateDM(userID)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	h.svc.WebView.SetTitle(friend.Name)

	state := h.session(r)
	ctx := r.Context()

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

	if htmx.IsHTMX(r) {
		w.Header().Set("Hx-Trigger", "close-modal")
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
