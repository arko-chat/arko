package matrix

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/arko-chat/arko/internal/models"
	"github.com/puzpuzpuz/xsync/v4"
	"github.com/tidwall/btree"
	"golang.org/x/sync/singleflight"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type MessageTree struct {
	mu sync.RWMutex
	*btree.BTreeG[models.Message]
	nonces *xsync.Map[string, models.Message]

	matrixSession *MatrixSession

	evtListenerId atomic.Uint64

	listening      atomic.Bool
	listenerCtx    context.Context
	listenerCancel context.CancelFunc
	listenerCh     chan MessageTreeEvent

	roomID      string
	isEncrypted bool

	chronoCache *xsync.Map[string, cacheEntry[[]models.Message]]
	chronoSfg   *singleflight.Group

	wg sync.WaitGroup
}

type MessageTreeEventType uint32

const (
	AddEvent MessageTreeEventType = iota
	RemoveEvent
	UpdateEvent
)

type MessageTreeEvent struct {
	EventType   MessageTreeEventType
	UpdateNonce string
	Message     models.Message
}

type Neighbors struct {
	Prev *models.Message
	Next *models.Message
}

func byTimestamp(a, b models.Message) bool {
	if a.Timestamp.Equal(b.Timestamp) {
		return a.ID < b.ID
	}
	return a.Timestamp.Before(b.Timestamp)
}

func newMessageTree(mxSession *MatrixSession, roomID string) *MessageTree {
	return &MessageTree{
		BTreeG: btree.NewBTreeGOptions(byTimestamp, btree.Options{
			NoLocks: true,
		}),
		matrixSession: mxSession,
		nonces:        xsync.NewMap[string, models.Message](),
		roomID:        roomID,
		chronoCache:   xsync.NewMap[string, cacheEntry[[]models.Message]](),
		chronoSfg:     &singleflight.Group{},
	}
}

func (t *MessageTree) Initialize(ctx context.Context) {
	var encEvt event.EncryptionEventContent
	err := t.matrixSession.GetClient().StateEvent(
		ctx, id.RoomID(t.roomID), event.StateEncryption, "", &encEvt,
	)
	t.isEncrypted = err == nil && encEvt.Algorithm != ""

	t.PopulateTree(ctx, "", "", 50)
}

func (t *MessageTree) PopulateTree(ctx context.Context, from, to string, limit int) {
	userID := id.UserID(t.matrixSession.id)
	roomID := t.roomID

	rid := id.RoomID(roomID)

	client := t.matrixSession.GetClient()
	cryptoHelper := t.matrixSession.GetCryptoHelper()

	requestedSessions := xsync.NewMap[id.SessionID, struct{}]()

	_, _ = t.matrixSession.keyBackupMgr.RestoreRoomKeys(ctx, rid)
	resp, err := client.Messages(ctx, rid, from, to, mautrix.DirectionBackward, nil, limit)
	if err != nil {
		t.matrixSession.logger.Error("failed to get messages", "roomID", roomID, "error", err)
		return
	}

	var wg sync.WaitGroup
	for _, evt := range resp.Chunk {
		wg.Go(func() {
			if evt.Type != event.EventEncrypted {
				if msg := t.eventToMessage(evt); msg != nil {
					t.Set(*msg)
				}
				return
			}

			_ = evt.Content.ParseRaw(evt.Type)
			encContent, ok := evt.Content.Parsed.(*event.EncryptedEventContent)
			if !ok {
				return
			}

			decrypted, decErr := cryptoHelper.Decrypt(ctx, evt)
			if decErr == nil {
				if msg := t.eventToMessage(decrypted); msg != nil {
					t.Set(*msg)
				}
				return
			}

			if errors.Is(decErr, crypto.ErrNoSessionFound) {
				if _, ok := requestedSessions.Load(encContent.SessionID); !ok {
					requestedSessions.Store(encContent.SessionID, struct{}{})

					t.matrixSession.logger.Debug("requesting session (first time)", "sessionID", encContent.SessionID)

					cryptoHelper.RequestSession(
						ctx, rid, encContent.SenderKey,
						encContent.SessionID, evt.Sender, encContent.DeviceID,
					)

					go func(e *event.Event, content *event.EncryptedEventContent) {
						if cryptoHelper.WaitForSession(ctx, rid, content.SenderKey, content.SessionID, 20*time.Second) {
							dec, retryErr := cryptoHelper.Decrypt(ctx, e)
							if retryErr == nil {
								if msg := t.eventToMessage(dec); msg != nil {
									t.Set(*msg)
								}
								_ = t.matrixSession.keyBackupMgr.BackupRoomKeys(ctx, rid, userID, content.SessionID)
							} else {
								t.Set(t.undecryptableMessage(e))
							}
						}
					}(evt, encContent)
				} else {
					t.matrixSession.logger.Debug("skipping redundant session request", "sessionID", encContent.SessionID)
				}
			}
		})
	}

	wg.Wait()
}

func (t *MessageTree) Listen(ctx context.Context, cb func(MessageTreeEvent)) {
	if t.listening.Swap(true) {
		return
	}

	t.matrixSession.logger.Debug(
		"listening to message tree",
		"roomID", t.roomID,
	)

	if t.listenerCancel != nil {
		t.listenerCancel()
	}

	t.listenerCh = make(chan MessageTreeEvent, 256)

	treeCtx, cancel := context.WithCancel(ctx)
	t.listenerCtx = treeCtx
	t.listenerCancel = cancel

	evtCh, evtChId := t.matrixSession.listenEvents()
	t.evtListenerId.Store(evtChId)

	t.wg.Go(func() {
		t.matrixSession.logger.Debug(
			"starting matrix receiver goroutine",
			"roomID", t.roomID,
		)

		for {
			select {
			case <-treeCtx.Done():
				return
			case evt, ok := <-evtCh:
				if !ok {
					return
				}
				if evt == nil || evt.RoomID != id.RoomID(t.roomID) {
					if evt != nil {
						t.matrixSession.logger.Debug(
							"received matrix event but not from room",
							"roomID", t.roomID,
							"evtRoomID", evt.RoomID,
						)
					}

					continue
				}

				t.matrixSession.logger.Debug(
					"received matrix event from room",
					"roomID", t.roomID,
				)

				msg := t.eventToMessage(evt)
				if msg != nil {
					t.matrixSession.logger.Debug(
						"parsed message",
						"roomID", t.roomID,
						"msgID", evt.ID,
						"content", evt.Content,
						"nonce", msg.Nonce,
					)
					t.Set(*msg)
				}
			}
		}
	})

	t.wg.Go(func() {
		t.matrixSession.logger.Debug(
			"starting callback transmitter goroutine",
			"roomID", t.roomID,
		)

		for {
			select {
			case <-treeCtx.Done():
				return
			case evt, ok := <-t.listenerCh:
				if !ok {
					return
				}

				t.matrixSession.logger.Debug(
					"received message tree changes",
					"roomID", t.roomID,
					"type", evt.EventType,
				)

				if cb != nil {
					cb(evt)
				}
			}
		}
	})
}

func (t *MessageTree) Close() {
	if !t.listening.Swap(false) {
		return
	}

	if t.listenerCancel != nil {
		t.listenerCancel()
	}

	oldId := t.evtListenerId.Swap(0)
	t.matrixSession.closeEventListener(oldId)

	t.listenerCtx = nil
	t.listenerCancel = nil
	close(t.listenerCh)

	t.wg.Wait()
}

func (t *MessageTree) SendMessage(ctx context.Context, body string) error {
	nonce, err := generateNonce()
	if err != nil {
		return err
	}

	t.Set(t.defaultMessage(ctx, body, nonce))

	rid := id.RoomID(t.roomID)
	client := t.matrixSession.GetClient()
	content := &event.MessageEventContent{
		MsgType: event.MsgText,
		Body:    body,
	}

	if t.isEncrypted {
		t.shareGroupSession(ctx)

		encrypted, err := t.matrixSession.GetCryptoHelper().Encrypt(
			ctx, rid, event.EventMessage, content,
		)
		if err != nil {
			return fmt.Errorf("encrypt: %w", err)
		}

		_, err = client.SendMessageEvent(
			ctx, rid, event.EventEncrypted, encrypted,
			mautrix.ReqSendEvent{TransactionID: nonce},
		)
		return err
	}

	_, err = client.SendMessageEvent(
		ctx, rid, event.EventMessage, content,
		mautrix.ReqSendEvent{TransactionID: nonce},
	)
	return err
}

func (t *MessageTree) Set(m models.Message) (models.Message, bool) {
	m.RoomID = t.roomID

	replacedPending := false
	replacedNonce := ""

	t.mu.Lock()
	// if message has nonce, it is meant to replace an existing message with
	// "pending-<nonce>" as ID
	if m.Nonce != "" {
		if existing, ok := t.nonces.Load(m.Nonce); ok {
			t.matrixSession.logger.Debug(
				"nonce found",
				"nonce", m.Nonce,
				"existing", existing,
			)
			t.BTreeG.Delete(existing)

			t.matrixSession.logger.Debug(
				"finished processing nonce",
			)
			replacedNonce = existing.ID
			replacedPending = true
			t.nonces.Delete(m.Nonce)
		}
		m.Nonce = ""
	} else if strings.HasPrefix(m.ID, "pending-") {
		t.matrixSession.logger.Debug(
			"pending found",
			"id", m.ID,
		)
		t.nonces.Store(m.ID, m)
	}

	_, replaced := t.BTreeG.Set(m)
	t.chronoCache.Delete("chrono_list")
	t.mu.Unlock()

	t.matrixSession.logger.Debug(
		"btree set",
		"replaced", replaced,
	)

	t.matrixSession.logger.Debug(
		"if stuck here, it's deadlock",
	)

	t.matrixSession.logger.Debug(
		"check if listening",
	)

	if t.listening.Load() {
		var treeEvt MessageTreeEvent
		if !replaced && !replacedPending {
			treeEvt = MessageTreeEvent{
				Message:   m,
				EventType: AddEvent,
			}
		} else {
			treeEvt = MessageTreeEvent{
				Message:     m,
				UpdateNonce: replacedNonce,
				EventType:   UpdateEvent,
			}
		}

		if t.listening.Load() {
			select {
			case t.listenerCh <- treeEvt:
				t.matrixSession.logger.Debug(
					"sent event to listener channel",
					"id", m.ID,
				)
			case <-t.listenerCtx.Done():
				t.matrixSession.logger.Debug("listener context done, skipping send")
			default:
				// Fallback: If channel is full, send in goroutine to prevent tree deadlock
				go func(evt MessageTreeEvent) {
					select {
					case t.listenerCh <- evt:
					case <-time.After(5 * time.Second):
						t.matrixSession.logger.Warn("dropped message tree event due to timeout")
					}
				}(treeEvt)
			}
		}
	}

	return m, replaced
}

func (t *MessageTree) GetNeighbors(m models.Message) Neighbors {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var n Neighbors
	iter := t.Iter()

	if !iter.Seek(m) {
		return n
	}

	if iter.Prev() {
		v := iter.Item()
		n.Prev = &v
		iter.Next()
	}

	iter.Seek(m)
	if iter.Next() {
		v := iter.Item()
		n.Next = &v
	}

	return n
}

func (t *MessageTree) Chronological() []models.Message {
	const cacheKey = "chrono_list"

	msgs, _ := cachedSingle(t.chronoCache, t.chronoSfg, cacheKey, func() ([]models.Message, error) {
		t.mu.RLock()
		defer t.mu.RUnlock()

		items := make([]models.Message, 0, t.Len())
		t.Reverse(func(item models.Message) bool {
			items = append(items, item)
			return true
		})
		return items, nil
	})

	return msgs
}

func (t *MessageTree) eventToMessage(
	evt *event.Event,
) *models.Message {
	content, ok := evt.Content.Parsed.(*event.MessageEventContent)
	if !ok {
		return nil
	}

	profile, _ := t.matrixSession.GetUserProfile(context.Background(), string(evt.Sender))

	safeId := safeHashClass(evt.ID.String())

	return &models.Message{
		ID:        safeId,
		Content:   content.Body,
		Author:    profile,
		Timestamp: time.UnixMilli(evt.Timestamp),
		RoomID:    t.roomID,
		Nonce:     evt.Unsigned.TransactionID,
	}
}

func (t *MessageTree) defaultMessage(ctx context.Context, content, nonce string) models.Message {
	currUser, _ := t.matrixSession.GetUserProfile(ctx, t.matrixSession.id)
	return models.Message{
		ID:        nonce,
		Content:   content,
		Author:    currUser,
		Timestamp: time.Now(),
		RoomID:    t.roomID,
	}
}

func (t *MessageTree) undecryptableMessage(
	evt *event.Event,
) models.Message {
	return models.Message{
		ID:      safeHashClass(evt.ID.String()),
		Content: "🔒 Unable to decrypt this message.",
		Author: models.User{
			ID:   evt.Sender.String(),
			Name: evt.Sender.Localpart(),
			Avatar: fmt.Sprintf(
				"https://api.dicebear.com/7.x/avataaars/svg?seed=%s",
				evt.Sender.Localpart(),
			),
			Status: models.StatusOnline,
		},
		Timestamp: time.UnixMilli(evt.Timestamp),
		RoomID:    t.roomID,
	}
}

func (t *MessageTree) shareGroupSession(ctx context.Context) {
	rid := id.RoomID(t.roomID)
	machine := t.matrixSession.GetCryptoHelper().Machine()
	if machine == nil {
		return
	}

	members, err := t.matrixSession.GetClient().Members(ctx, rid)
	if err != nil {
		return
	}

	var memberIDs []id.UserID
	for _, evt := range members.Chunk {
		c, ok := evt.Content.Parsed.(*event.MemberEventContent)
		if !ok || c.Membership != event.MembershipJoin {
			continue
		}
		memberIDs = append(memberIDs, id.UserID(evt.GetStateKey()))
	}

	if err := machine.ShareGroupSession(ctx, rid, memberIDs); err != nil {
		t.matrixSession.logger.Warn("share group session failed",
			"user", t.matrixSession.id,
			"room", rid,
			"err", err,
		)
	}
}
