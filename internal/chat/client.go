package chat

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 4096
)

// Client is a single WebSocket connection associated with one conversation.
type Client struct {
	hub            *Hub
	conn           *websocket.Conn
	conversationID string
	clerkID        string
	send           chan []byte
}

// inboundMsg is the shape accepted from the client.
// Type "message" (or omitted) carries a Body to persist.
// Type "typing" is a lightweight signal — no body required.
type inboundMsg struct {
	Type string `json:"type"`
	Body string `json:"body"`
}

// ReadPump pumps messages from the WebSocket to the hub.
// onMessage is called for every new chat message to persist and broadcast.
// onTyping is called when the client sends a typing indicator.
func (c *Client) ReadPump(onMessage func(body string), onTyping func()) {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("ws read error: conv=%s clerk=%s: %v", c.conversationID, c.clerkID, err)
			}
			break
		}
		var in inboundMsg
		if jsonErr := json.Unmarshal(raw, &in); jsonErr != nil {
			continue
		}
		switch in.Type {
		case "typing":
			onTyping()
		default:
			if in.Body != "" {
				onMessage(in.Body)
			}
		}
	}
}

// WritePump pumps messages from the hub's send channel to the WebSocket.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
