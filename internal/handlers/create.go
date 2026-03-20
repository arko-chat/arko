package handlers

import (
	"net/http"

	"github.com/arko-chat/arko/components/ui"
	"github.com/go-chi/chi/v5"
)

type CreateSpaceRequest struct {
	Name   string `form:"name"`
	Topic  string `form:"topic"`
	Public bool   `form:"public"`
}

type CreateChannelRequest struct {
	Name   string `form:"name"`
	Topic  string `form:"topic"`
	Public bool   `form:"public"`
}

func (h *Handler) HandleCreateSpace(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.clientError(w, r, http.StatusBadRequest, "Invalid form data")
		return
	}

	name := r.FormValue("name")
	if name == "" {
		h.clientError(w, r, http.StatusBadRequest, "Space name is required")
		return
	}

	topic := r.FormValue("topic")
	public := r.FormValue("public") == "true"

	space, err := h.svc.Spaces.CreateSpace(name, topic, public)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	h.htmxRedirect(w, "/spaces/"+space.ID)
}

func (h *Handler) HandleCreateChannel(w http.ResponseWriter, r *http.Request) {
	spaceID := chi.URLParam(r, "spaceID")

	if err := r.ParseForm(); err != nil {
		h.clientError(w, r, http.StatusBadRequest, "Invalid form data")
		return
	}

	name := r.FormValue("name")
	if name == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = ui.Alert("Channel name is required").Render(r.Context(), w)
		return
	}

	topic := r.FormValue("topic")
	public := r.FormValue("public") == "true"

	channel, err := h.svc.Spaces.CreateChannel(spaceID, name, topic, public)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	h.htmxRedirect(w, "/spaces/"+spaceID+"/channels/"+channel.ID)
}
