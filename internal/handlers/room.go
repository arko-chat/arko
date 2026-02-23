package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/arko-chat/arko/internal/ws"
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
	client := &ws.Client{
		Hub:    hub,
		RoomID: roomID,
		UserID: state.UserID,
		Conn:   conn,
		Send:   make(chan []byte, 64),
	}

	hub.Register(roomID, client)
	go client.WritePump()

	client.ReadPump(func(uid, content string) {
		if strings.TrimSpace(content) == "" {
			return
		}

		author, err := h.svc.User.GetCurrentUser(r.Context())
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
			context.Background(),
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
