package ws

import (
	"log/slog"
	"sync"
)

type BaseHub struct {
	mu      sync.RWMutex
	buckets map[string]map[WSClient]struct{}
	Logger  *slog.Logger
}

func NewBaseHub(logger *slog.Logger) *BaseHub {
	return &BaseHub{
		buckets: make(map[string]map[WSClient]struct{}),
		Logger:  logger,
	}
}

func (h *BaseHub) Register(key string, c WSClient) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.buckets[key] == nil {
		h.buckets[key] = make(map[WSClient]struct{})
	}
	h.buckets[key][c] = struct{}{}
}

func (h *BaseHub) Unregister(key string, c WSClient) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.buckets[key]; !ok {
		return
	}
	delete(h.buckets[key], c)
	close(c.GetSend())
	if len(h.buckets[key]) == 0 {
		delete(h.buckets, key)
	}
}

func (h *BaseHub) Send(key string, data []byte, onDrop func(c WSClient)) {
	if data == nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for c := range h.buckets[key] {
		select {
		case c.GetSend() <- data:
		default:
			if onDrop != nil {
				onDrop(c)
			}
		}
	}
}

func (h *BaseHub) Count(key string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.buckets[key])
}

func (h *BaseHub) Broadcast(key string, data []byte) {
	h.Logger.Warn("Broadcast unimplemented")
}

func (h *BaseHub) Push(key string, data []byte) {
	h.Logger.Warn("Push unimplemented")
}
