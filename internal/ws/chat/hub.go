package chatws

import (
	"log/slog"

	"github.com/arko-chat/arko/internal/ws"
)

var _ ws.WSHub = (*Hub)(nil)

type Hub struct {
	*ws.BaseHub
}

func NewHub(logger *slog.Logger) *Hub {
	return &Hub{BaseHub: ws.NewBaseHub(logger)}
}

func (h *Hub) Register(roomID string, c ws.WSClient) {
	client, ok := c.(*Client)
	if !ok {
		return
	}

	h.BaseHub.Register(roomID, client)
	h.Logger.Debug("ws register",
		"room", roomID,
		"user", client.UserID,
		"clients", h.Count(roomID),
	)
}

func (h *Hub) Unregister(roomID string, c ws.WSClient) {
	client, ok := c.(*Client)
	if !ok {
		return
	}

	h.BaseHub.Unregister(roomID, client)
	h.Logger.Debug("ws unregister", "room", roomID, "user", client.UserID)
}

func (h *Hub) Broadcast(roomID string, data []byte) {
	sent := 0
	h.BaseHub.Send(roomID, data, func(_ ws.WSClient) {
		h.Logger.Warn("ws dropped message", "room", roomID)
	})
	h.Logger.Debug("ws broadcast", "room", roomID, "recipients", sent)
}
