package ws

import (
	"log/slog"
	"sync"
)

type Hub struct {
	mu      sync.RWMutex
	clients map[string]map[*Client]struct{}
	logger  *slog.Logger
}

func NewHub(logger *slog.Logger) *Hub {
	return &Hub{
		clients: make(map[string]map[*Client]struct{}),
		logger:  logger,
	}
}

func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.clients[c.UserID] == nil {
		h.clients[c.UserID] = make(map[*Client]struct{})
	}
	h.clients[c.UserID][c] = struct{}{}
	h.logger.Debug("ws register", "user", c.UserID)
}

func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	bucket, ok := h.clients[c.UserID]
	if !ok {
		return
	}
	if _, exists := bucket[c]; !exists {
		return
	}
	delete(bucket, c)
	close(c.send)
	if len(bucket) == 0 {
		delete(h.clients, c.UserID)
	}
	h.logger.Debug("ws unregister", "user", c.UserID)
}

func (h *Hub) Push(userID string, data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for c := range h.clients[userID] {
		select {
		case c.send <- data:
		default:
			h.logger.Warn("ws push dropped", "user", userID)
		}
	}
}

func (h *Hub) BroadcastToRoom(roomID string, data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, bucket := range h.clients {
		for c := range bucket {
			if c.GetActiveRoom() != roomID {
				continue
			}
			select {
			case c.send <- data:
			default:
				h.logger.Warn("ws broadcast dropped", "room", roomID)
			}
		}
	}
}
