package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/arko-chat/arko/internal/ws"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (h *Handler) HandleWS(w http.ResponseWriter, r *http.Request) {
	state := h.session(r)
	userID := state.UserID

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	client := ws.NewClient(h.hub, conn, userID)
	defer client.Close()

	h.hub.Register(client)
	go client.WritePump()

	h.svc.Verification.ListenVerifyEvents(r.Context())

	client.ReadPump(func(ctx context.Context, raw []byte) {
		var msg ws.ClientRequest
		if err := json.Unmarshal(raw, &msg); err != nil {
			return
		}

		switch msg.Action {
		case "SUBSCRIBE_ROOM":
			client.SetActiveRoom(msg.RoomID)
		case "SAS_CONFIRM":
			if err := h.svc.Verification.ConfirmVerification(ctx); err != nil {
				h.logger.Error("ws SAS_CONFIRM failed", "err", err)
			}
		case "SAS_CANCEL":
			if err := h.svc.Verification.CancelVerification(ctx); err != nil {
				h.logger.Error("ws SAS_CANCEL failed", "err", err)
			}
			h.hub.Push(userID, ws.RedirectMessage("/verify"))
		case "CONFIRM_QR":
			if err := h.svc.Verification.ConfirmQRVerification(ctx); err != nil {
				h.logger.Error("ws CONFIRM_QR failed", "err", err)
			}
		case "ROOM_MESSAGE":
			if strings.TrimSpace(msg.Message) == "" {
				return
			}
			author, err := h.svc.User.GetCurrentUser()
			if err != nil {
				return
			}
			if err := h.svc.Chat.SendRoomMessage(msg.RoomID, author, msg.Message); err != nil {
				h.logger.Error("ws chat send failed", "err", err)
			}
		}
	})
}
