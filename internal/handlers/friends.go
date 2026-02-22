package handlers

import (
	"net/http"

	"github.com/arko-chat/arko/components/features/friends"
	"github.com/arko-chat/arko/components/sidebar"
	"github.com/arko-chat/arko/internal/htmx"
	friendspage "github.com/arko-chat/arko/pages/friends"
)

func (h *Handler) HandleFriends(w http.ResponseWriter, r *http.Request) {
	state := h.session(r)
	ctx := r.Context()

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

	fl, _ := h.svc.Friends.ListFriends(ctx)

	if htmx.IsHTMX(r) {
		if err := friendspage.Content(user, spaces, fl).Render(ctx, w); err != nil {
			h.serverError(w, r, err)
		}
		return
	}

	if err := friendspage.Page(state, user, spaces, fl).Render(ctx, w); err != nil {
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

	_, err := h.svc.Friends.FilterFriends(r.Context(), filter)
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

	filtered, err := h.svc.Friends.SearchFriends(r.Context(), query)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	if err := sidebar.FriendsList(filtered).Render(r.Context(), w); err != nil {
		h.serverError(w, r, err)
	}
}
