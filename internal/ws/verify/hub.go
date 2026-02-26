package verifyws

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

func (h *Hub) Register(userID string, c ws.WSClient) {
	client, ok := c.(*Client)
	if !ok {
		return
	}

	h.BaseHub.Register(userID, client)
	h.Logger.Debug("verify ws register",
		"user", userID,
		"clients", h.Count(userID),
	)
}

func (h *Hub) Unregister(userID string, c ws.WSClient) {
	client, ok := c.(*Client)
	if !ok {
		return
	}

	h.BaseHub.Unregister(userID, client)
	h.Logger.Debug("verify ws unregister", "user", userID)
}

func (h *Hub) Push(userID string, data []byte) {
	h.BaseHub.Send(userID, data, func(_ ws.WSClient) {
		h.Logger.Warn("verify ws dropped message", "user", userID)
	})
}
