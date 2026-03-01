package handlers

import (
	"fmt"
	"net/http"

	"github.com/arko-chat/arko/components"
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

	tree, err := h.svc.Chat.GetRoomMessageTree(channelID)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	fl, _ := h.svc.Friends.ListFriends()

	props := channelspage.ContentProps{
		User:        user,
		FriendsList: fl,
		Spaces:      spaces,
		SpaceDetail: detail,
		Channel:     ch,
		Tree:        tree,
		RoomID:      ch.ID,
	}

	h.svc.WebView.SetTitle(fmt.Sprintf("#%s", ch.Name))

	if htmx.IsHTMX(r) {
		if err := channelspage.Content(props).Render(ctx, w); err != nil {
			h.serverError(w, r, err)
		}
		return
	}

	if err := channelspage.Page(channelspage.PageProps{
		PageProps: components.PageProps{
			State: state,
			Title: h.svc.WebView.GetTitle(),
		},
		ContentProps: props,
	}).Render(ctx, w); err != nil {
		h.serverError(w, r, err)
	}
}
