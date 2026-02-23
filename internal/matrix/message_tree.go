package matrix

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/arko-chat/arko/internal/models"
	"github.com/puzpuzpuz/xsync/v4"
	"github.com/tidwall/btree"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type MessageTree struct {
	*btree.BTreeG[models.Message]
	nonces *xsync.Map[string, models.Message]

	matrixSession *MatrixSession

	evtListenerId atomic.Uint64

	listening      atomic.Bool
	listenerCtx    context.Context
	listenerCancel context.CancelFunc
	listenerCh     chan MessageTreeEvent

	roomID string

	wg sync.WaitGroup
}

type MessageTreeEventType uint32

const (
	AddEvent MessageTreeEventType = iota
	RemoveEvent
	UpdateEvent
)

type MessageTreeEvent struct {
	EventType MessageTreeEventType
	Message   models.Message
	Neighbors Neighbors
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
		BTreeG:        btree.NewBTreeG(byTimestamp),
		matrixSession: mxSession,
		nonces:        xsync.NewMap[string, models.Message](),
		roomID:        roomID,
	}
}

func (t *MessageTree) Initialize(ctx context.Context) {
	t.PopulateTree(ctx, "", "", 50)
}

func (t *MessageTree) PopulateTree(ctx context.Context, from, to string, limit int) {
	userID := id.UserID(t.matrixSession.id)
	roomID := t.roomID

	actualRoomID := decodeRoomID(roomID)
	rid := id.RoomID(actualRoomID)

	client := t.matrixSession.GetClient()
	cryptoHelper := t.matrixSession.GetCryptoHelper()

	requestedSessions := xsync.NewMap[id.SessionID, struct{}]()

	_, _ = t.matrixSession.keyBackupMgr.RestoreRoomKeys(ctx, rid)
	resp, err := client.Messages(ctx, rid, from, to, mautrix.DirectionBackward, nil, limit)
	if err != nil {
		t.matrixSession.logger.Error("failed to get messages", "roomID", roomID, "error", err)
		return
	}

	for _, evt := range resp.Chunk {
		if evt.Type != event.EventEncrypted {
			if msg := t.eventToMessage(evt); msg != nil {
				t.Set(*msg)
			}
			continue
		}

		_ = evt.Content.ParseRaw(evt.Type)
		encContent, ok := evt.Content.Parsed.(*event.EncryptedEventContent)
		if !ok {
			continue
		}

		decrypted, decErr := cryptoHelper.Decrypt(ctx, evt)
		if decErr == nil {
			if msg := t.eventToMessage(decrypted); msg != nil {
				t.Set(*msg)
			}
			continue
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
						}
					}
				}(evt, encContent)
			} else {
				t.matrixSession.logger.Debug("skipping redundant session request", "sessionID", encContent.SessionID)
			}
		}
	}
}

func (t *MessageTree) Listen(ctx context.Context, cb func(MessageTreeEvent)) {
	if t.listening.Swap(true) {
		return
	}

	if t.listenerCancel != nil {
		t.listenerCancel()
	}

	t.listenerCh = make(chan MessageTreeEvent)

	treeCtx, cancel := context.WithCancel(ctx)
	t.listenerCtx = treeCtx
	t.listenerCancel = cancel

	evtCh, evtChId := t.matrixSession.listenEvents()
	t.evtListenerId.Store(evtChId)

	t.wg.Go(func() {
		for {
			select {
			case <-treeCtx.Done():
				return
			case evt, ok := <-evtCh:
				if !ok {
					return
				}
				if evt == nil || evt.RoomID != id.RoomID(t.roomID) {
					continue
				}

				msg := t.eventToMessage(evt)
				if cb != nil && msg != nil {

					cb(MessageTreeEvent{
						EventType: AddEvent,
						Message:   *msg,
						Neighbors: t.GetNeighbors(*msg),
					})
				}
			case evt, ok := <-t.listenerCh:
				if !ok {
					return
				}
				if evt.Message.RoomID != t.roomID {
					continue
				}
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

func (t *MessageTree) SendMessage(
	ctx context.Context,
	body string,
) error {
	client := t.matrixSession.GetClient()

	nonce, err := generateNonce()
	if err != nil {
		return err
	}

	t.Set(t.defaultMessage(ctx, body, nonce))

	rid := id.RoomID(t.roomID)

	content := &event.MessageEventContent{
		MsgType: event.MsgText,
		Body:    body,
	}

	var encEvt event.EncryptionEventContent
	err = client.StateEvent(
		ctx, rid, event.StateEncryption, "", &encEvt,
	)
	if err == nil && encEvt.Algorithm != "" {
		machine := t.matrixSession.GetCryptoHelper().Machine()
		if machine != nil {
			members, memberErr := client.Members(ctx, rid)
			if memberErr == nil {
				var memberIDs []id.UserID
				for _, evt := range members.Chunk {
					c, ok := evt.Content.Parsed.(*event.MemberEventContent)
					if !ok || c.Membership != event.MembershipJoin {
						continue
					}
					memberIDs = append(
						memberIDs, id.UserID(evt.GetStateKey()),
					)
				}
				if shareErr := machine.ShareGroupSession(
					ctx, rid, memberIDs,
				); shareErr != nil {
					t.matrixSession.logger.Warn("share group session failed",
						"user", t.matrixSession.id,
						"room", rid,
						"err", shareErr,
					)
				}
			}
		}

		encrypted, encErr := t.matrixSession.GetCryptoHelper().Encrypt(
			ctx, rid, event.EventMessage, content,
		)
		if encErr != nil {
			return fmt.Errorf("encrypt: %w", encErr)
		}

		_, sendErr := client.SendMessageEvent(
			ctx, rid, event.EventEncrypted, encrypted,
			mautrix.ReqSendEvent{TransactionID: nonce},
		)
		if sendErr != nil {
			return sendErr
		}
		return nil
	}

	_, err = client.SendMessageEvent(
		ctx, rid, event.EventMessage, content,
		mautrix.ReqSendEvent{TransactionID: nonce},
	)
	if err != nil {
		return err
	}
	return nil
}

func (t *MessageTree) Set(m models.Message) (models.Message, bool) {
	m.RoomID = t.roomID

	replacedPending := false
	if existing, ok := t.nonces.Load(m.Nonce); ok {
		t.BTreeG.Delete(existing)
		m.Nonce = ""
		replacedPending = true
	} else if m.Nonce != "" {
		t.nonces.Store(m.Nonce, m)
	}

	msg, replaced := t.BTreeG.Set(m)

	if t.listening.Load() {
		if !replaced && !replacedPending {
			t.listenerCh <- MessageTreeEvent{
				Message:   msg,
				EventType: AddEvent,
				Neighbors: t.GetNeighbors(msg),
			}
		} else {
			t.listenerCh <- MessageTreeEvent{
				Message:   msg,
				EventType: UpdateEvent,
				Neighbors: t.GetNeighbors(msg),
			}
		}
	}

	return msg, replaced
}

func (t *MessageTree) GetNeighbors(m models.Message) Neighbors {
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
	items := make([]models.Message, 0, t.Len())
	t.Reverse(func(item models.Message) bool {
		items = append(items, item)
		return true
	})
	return items
}

func (t *MessageTree) eventToMessage(
	evt *event.Event,
) *models.Message {
	content, ok := evt.Content.Parsed.(*event.MessageEventContent)
	if !ok {
		return nil
	}

	senderName := evt.Sender.Localpart()
	avatarURL := fmt.Sprintf(
		"https://api.dicebear.com/7.x/avataaars/svg?seed=%s",
		senderName,
	)

	profile, _ := t.matrixSession.GetClient().GetProfile(context.Background(), evt.Sender)
	if profile != nil {
		if profile.DisplayName != "" {
			senderName = profile.DisplayName
		}
		avatarURL = resolveContentURI(
			profile.AvatarURL, evt.Sender.Localpart(), "avataaars",
		)
	}

	safeId := safeHashClass(evt.ID.String())

	return &models.Message{
		ID:      safeId,
		Content: content.Body,
		Author: models.User{
			ID:     evt.Sender.String(),
			Name:   senderName,
			Avatar: avatarURL,
			Status: models.StatusOnline,
		},
		Timestamp: time.UnixMilli(evt.Timestamp),
		RoomID:    t.roomID,
		Nonce:     evt.Unsigned.TransactionID,
	}
}

func (t *MessageTree) defaultMessage(ctx context.Context, content, nonce string) models.Message {
	currUser, _ := t.matrixSession.GetUserProfile(ctx, t.matrixSession.id)
	return models.Message{
		Content:   content,
		Author:    currUser,
		Timestamp: time.Now(),
		RoomID:    t.roomID,
		Nonce:     nonce,
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
