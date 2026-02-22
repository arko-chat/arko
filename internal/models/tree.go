package models

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/puzpuzpuz/xsync/v4"
	"github.com/tidwall/btree"
)

type MessageTree struct {
	*btree.BTreeG[Message]

	idInc  atomic.Uint64
	ctx    *xsync.Map[uint64, context.Context]
	cancel *xsync.Map[uint64, context.CancelFunc]
	msgCh  *xsync.Map[uint64, chan Message]

	wg sync.WaitGroup
}

type Neighbors struct {
	Prev *Message
	Next *Message
}

func byTimestamp(a, b Message) bool {
	return a.Timestamp.Before(b.Timestamp)
}

func NewMessageTree() *MessageTree {
	return &MessageTree{
		BTreeG: btree.NewBTreeG(byTimestamp),
		msgCh:  xsync.NewMap[uint64, chan Message](),
		ctx:    xsync.NewMap[uint64, context.Context](),
		cancel: xsync.NewMap[uint64, context.CancelFunc](),
	}
}

func (t *MessageTree) getNewListenerID() uint64 {
	return t.idInc.Add(1)
}

func (t *MessageTree) Listen(ctx context.Context, cb func(Message)) uint64 {
	id := t.getNewListenerID()
	treeCtx, cancel := context.WithCancel(ctx)
	msgCh := make(chan Message)

	t.msgCh.Store(id, msgCh)
	t.ctx.Store(id, treeCtx)
	t.cancel.Store(id, cancel)

	t.wg.Go(func() {
		for {
			select {
			case <-treeCtx.Done():
				return
			case msg, ok := <-msgCh:
				if !ok {
					return
				}
				if cb != nil {
					cb(msg)
				}
			}
		}
	})

	return id
}

func (t *MessageTree) Close(id uint64) {
	t.cancel.Compute(id, func(v context.CancelFunc, loaded bool) (context.CancelFunc, xsync.ComputeOp) {
		if loaded && v != nil {
			v()
		}
		return nil, xsync.DeleteOp
	})
	t.ctx.Delete(id)
	t.msgCh.Compute(id, func(v chan Message, loaded bool) (chan Message, xsync.ComputeOp) {
		if loaded && v != nil {
			close(v)
		}
		return nil, xsync.DeleteOp
	})

	t.wg.Wait()
}

func (t *MessageTree) Set(m Message) (Message, bool) {
	go func() {
		t.msgCh.Range(func(_ uint64, value chan Message) bool {
			value <- m
			return true
		})
	}()
	return t.BTreeG.Set(m)
}

func (t *MessageTree) GetNeighbors(m Message) Neighbors {
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

func (t *MessageTree) Chronological() []Message {
	items := make([]Message, 0, t.Len())
	t.Reverse(func(item Message) bool {
		items = append(items, item)
		return true
	})
	return items
}
