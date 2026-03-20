package matrix

import (
	"sync"
	"time"

	"github.com/arko-chat/arko/internal/models"
)

type TypingEvent struct {
	RoomID      string
	TypingUsers []TypingUser
}

type TypingUser struct {
	UserID   string
	Name     string
	Avatar   string
	Deadline time.Time
}

type TypingTracker struct {
	mu          sync.RWMutex
	rooms       map[string]map[string]*TypingUser
	listeners   []chan TypingEvent
	listenerMu  sync.RWMutex
	currentUser string
}

func NewTypingTracker(currentUser string) *TypingTracker {
	return &TypingTracker{
		rooms:       make(map[string]map[string]*TypingUser),
		currentUser: currentUser,
	}
}

func (t *TypingTracker) SetTyping(roomID string, user models.User, typing bool, timeout time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.rooms[roomID] == nil {
		t.rooms[roomID] = make(map[string]*TypingUser)
	}

	if !typing {
		delete(t.rooms[roomID], user.ID)
	} else {
		deadline := time.Now().Add(timeout)
		t.rooms[roomID][user.ID] = &TypingUser{
			UserID:   user.ID,
			Name:     user.Name,
			Avatar:   user.Avatar,
			Deadline: deadline,
		}
	}

	go t.notifyListeners(roomID)
}

func (t *TypingTracker) GetTypingUsers(roomID string) []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	t.cleanupExpired(roomID)

	room, ok := t.rooms[roomID]
	if !ok {
		return nil
	}

	var names []string
	for _, u := range room {
		if u.UserID != t.currentUser {
			names = append(names, u.Name)
		}
	}
	return names
}

func (t *TypingTracker) cleanupExpired(roomID string) {
	room, ok := t.rooms[roomID]
	if !ok {
		return
	}

	now := time.Now()
	for id, u := range room {
		if now.After(u.Deadline) {
			delete(room, id)
		}
	}
}

func (t *TypingTracker) Listen() <-chan TypingEvent {
	t.listenerMu.Lock()
	defer t.listenerMu.Unlock()

	ch := make(chan TypingEvent, 16)
	t.listeners = append(t.listeners, ch)
	return ch
}

func (t *TypingTracker) CloseListener(ch <-chan TypingEvent) {
	t.listenerMu.Lock()
	defer t.listenerMu.Unlock()

	for i, listener := range t.listeners {
		if listener == ch {
			t.listeners = append(t.listeners[:i], t.listeners[i+1:]...)
			close(listener)
			break
		}
	}
}

func (t *TypingTracker) notifyListeners(roomID string) {
	t.listenerMu.RLock()
	listeners := make([]chan TypingEvent, len(t.listeners))
	copy(listeners, t.listeners)
	t.listenerMu.RUnlock()

	evt := TypingEvent{
		RoomID:      roomID,
		TypingUsers: t.getTypingUsersForRoom(roomID),
	}

	for _, ch := range listeners {
		select {
		case ch <- evt:
		default:
		}
	}
}

func (t *TypingTracker) getTypingUsersForRoom(roomID string) []TypingUser {
	t.mu.RLock()
	defer t.mu.RUnlock()

	t.cleanupExpired(roomID)

	room, ok := t.rooms[roomID]
	if !ok {
		return nil
	}

	var users []TypingUser
	for _, u := range room {
		if u.UserID != t.currentUser {
			users = append(users, *u)
		}
	}
	return users
}
