package ws

import (
	"log/slog"
	"sync"
)

type Hub struct {
	mu     sync.RWMutex
	rooms  map[string]map[*Client]struct{}
	logger *slog.Logger
}

func NewHub(logger *slog.Logger) *Hub {
	return &Hub{
		rooms:  make(map[string]map[*Client]struct{}),
		logger: logger,
	}
}

func (h *Hub) Register(roomID string, c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.rooms[roomID] == nil {
		h.rooms[roomID] = make(map[*Client]struct{})
	}
	h.rooms[roomID][c] = struct{}{}
	h.logger.Debug("ws register",
		"room", roomID,
		"user", c.UserID,
		"clients", len(h.rooms[roomID]),
	)
}

func (h *Hub) Unregister(roomID string, c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.rooms[roomID]; !ok {
		return
	}
	delete(h.rooms[roomID], c)
	close(c.Send)
	if len(h.rooms[roomID]) == 0 {
		delete(h.rooms, roomID)
	}
	h.logger.Debug("ws unregister", "room", roomID, "user", c.UserID)
}

func (h *Hub) Broadcast(roomID string, data []byte) {
	if data == nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	sent := 0
	for c := range h.rooms[roomID] {
		select {
		case c.Send <- data:
			sent++
		default:
			h.logger.Warn("ws dropped message",
				"room", roomID,
				"user", c.UserID,
			)
		}
	}

	h.logger.Debug("ws broadcast",
		"room", roomID,
		"recipients", sent,
	)
}
