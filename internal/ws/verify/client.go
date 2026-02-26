package verifyws

import (
	"encoding/json"
	"time"

	"github.com/arko-chat/arko/internal/ws"
)

var _ ws.WSClient = (*Client)(nil)

type Client struct {
	*ws.BaseClient
	Hub    ws.WSHub
	UserID string
}

type verifyIncomingMessage struct {
	Action string `json:"action"`
}

type MessageHandler func(userID, action string)

func (c *Client) ReadPump(onMessage MessageHandler) {
	defer func() {
		c.Hub.Unregister(c.UserID, c)
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

		var msg verifyIncomingMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}
		if msg.Action == "" {
			continue
		}

		onMessage(c.UserID, msg.Action)
	}
}
