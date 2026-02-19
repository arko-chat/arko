package handlers

import (
	"net/http"

	"github.com/arko-chat/arko/components/features/friends"
	"github.com/arko-chat/arko/components/pages"
	"github.com/arko-chat/arko/components/sidebar"
)

func (h *Handler) HandleFriends(w http.ResponseWriter, r *http.Request) {
	state := h.state(r)
	ctx := r.Context()

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

	fl, err := h.svc.ListFriends(ctx, state.UserID)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	if err := pages.FriendsPageContent(user, spaces, fl).Render(ctx, w); err != nil {
		h.serverError(w, r, err)
	}
	return
}

func (h *Handler) HandleFriendsFilter(
	w http.ResponseWriter,
	r *http.Request,
) {
	state := h.state(r)
	filter := r.URL.Query().Get("filter")
	if filter == "" {
		filter = "online"
	}

	_, err := h.svc.FilterFriends(r.Context(), state.UserID, filter)
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
	state := h.state(r)
	query := r.FormValue("search")

	filtered, err := h.svc.SearchFriends(r.Context(), state.UserID, query)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	if err := sidebar.FriendsList(filtered).Render(r.Context(), w); err != nil {
		h.serverError(w, r, err)
	}
}
