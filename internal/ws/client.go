package ws

import (
	"time"

	"github.com/gorilla/websocket"
)

type BaseClient struct {
	Conn *websocket.Conn
	Send chan []byte
}

func NewBaseClient(conn *websocket.Conn) *BaseClient {
	return &BaseClient{
		Conn: conn,
		Send: make(chan []byte, 256),
	}
}

func (c *BaseClient) GetSend() chan []byte {
	return c.Send
}

func (c *BaseClient) WritePump() {
	ticker := time.NewTicker(PingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(WriteWait))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(WriteWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
