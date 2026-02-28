package ws

import (
	"context"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	ctx          context.Context
	cancel       context.CancelFunc
	Hub          *Hub
	UserID       string
	activeRoom   string
	activeRoomMu sync.Mutex
	conn         *websocket.Conn
	send         chan []byte
	closeOnce    sync.Once
}

type ClientRequest struct {
	Action  string `json:"action"`
	RoomID  string `json:"roomID"`
	Message string `json:"message"`
}

func NewClient(hub *Hub, conn *websocket.Conn, userID string) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		ctx:    ctx,
		cancel: cancel,
		Hub:    hub,
		UserID: userID,
		conn:   conn,
		send:   make(chan []byte, 256),
	}
}

func (c *Client) Close() {
	c.closeOnce.Do(func() {
		c.cancel()

		if c.Hub != nil {
			c.Hub.Unregister(c)
		}

		c.conn.Close()
	})
}

func (c *Client) SetActiveRoom(roomID string) {
	c.activeRoomMu.Lock()
	defer c.activeRoomMu.Unlock()
	c.activeRoom = roomID
}

func (c *Client) GetActiveRoom() string {
	c.activeRoomMu.Lock()
	defer c.activeRoomMu.Unlock()
	return c.activeRoom
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(PingPeriod)
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {
		select {
		case <-c.ctx.Done():
			c.conn.SetWriteDeadline(time.Now().Add(WriteWait))
			c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			return
		case msg, ok := <-c.send:
			if !ok {
				c.conn.SetWriteDeadline(time.Now().Add(WriteWait))
				c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				return
			}
			c.conn.SetWriteDeadline(time.Now().Add(WriteWait))
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(WriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) ReadPump(onMessage func(ctx context.Context, raw []byte)) {
	defer c.Close()

	c.conn.SetReadLimit(MaxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(PongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(PongWait))
		return nil
	})

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		if len(raw) == 0 {
			continue
		}
		onMessage(c.ctx, raw)
	}
}
