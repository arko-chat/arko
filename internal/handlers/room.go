package handlers

import (
	"net/http"
	"strings"

	"github.com/arko-chat/arko/components/ui"
	"github.com/arko-chat/arko/internal/ws"
	chatws "github.com/arko-chat/arko/internal/ws/chat"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (h *Handler) HandleRoom(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "roomID")
	state := h.session(r)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	hub := h.svc.Chat.Hub()
	client := &chatws.Client{
		Hub:        hub,
		RoomID:     roomID,
		UserID:     state.UserID,
		BaseClient: ws.NewBaseClient(conn),
	}

	hub.Register(roomID, client)
	go client.WritePump()

	client.ReadPump(func(uid, content string) {
		if strings.TrimSpace(content) == "" {
			return
		}

		author, err := h.svc.User.GetCurrentUser()
		if err != nil {
			return
		}

		h.logger.Debug(
			"sending room message from ws pump",
			"roomID", roomID,
			"author", author,
			"content", content,
		)
		err = h.svc.Chat.SendRoomMessage(
			roomID,
			author,
			content,
		)
		if err != nil {
			h.logger.Error("send matrix message", "err", err)
			return
		}
	})
}

func (h *Handler) HandleRoomHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	roomID := chi.URLParam(r, "roomID")

	hasMore, err := h.svc.Chat.LoadRoomHistory(roomID, 30)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	if !hasMore {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if err := ui.MoreMessageScrollSensor(roomID, hasMore).Render(ctx, w); err != nil {
		h.serverError(w, r, err)
	}
}
