package chatws

import (
	"encoding/json"
	"time"

	"github.com/arko-chat/arko/internal/ws"
)

var _ ws.WSClient = (*Client)(nil)

type Client struct {
	*ws.BaseClient
	Hub    ws.WSHub
	RoomID string
	UserID string
}

type incomingMessage struct {
	Message string `json:"message"`
	Nonce   string `json:"nonce"`
}

type MessageHandler func(userID, content string)

func (c *Client) ReadPump(onMessage MessageHandler) {
	defer func() {
		c.Hub.Unregister(c.RoomID, c)
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(ws.MaxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(ws.PongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(ws.PongWait))
		return nil
	})

	for {
		_, raw, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}

		var msg incomingMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}
		if msg.Message == "" {
			continue
		}

		onMessage(c.UserID, msg.Message)
	}
}
