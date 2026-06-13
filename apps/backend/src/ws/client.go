package ws

import (
	"time"

	"github.com/gorilla/websocket"
)

const (
	// maxMessageSize bounds inbound frames; Yjs updates are typically small,
	// but full-document syncs of large canvases need headroom.
	maxMessageSize = 1 << 20 // 1 MiB
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
)

// Client is one WebSocket connection bound to a single room. All writes go
// through the buffered send channel and are flushed by writePump, because
// gorilla/websocket connections do not support concurrent writers.
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	room string
	send chan []byte
}

// readPump reads frames from the peer and hands them to the hub for relay to
// the rest of the room. It enforces the read limit and pong-based liveness
// deadline, and unregisters the client (which closes its send channel) when
// the connection drops.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		c.hub.broadcast <- frame{room: c.room, sender: c, data: data}
	}
}

// writePump is the sole writer on the connection: it drains the send channel
// as binary frames, emits keepalive pings, and sends a close frame when the
// hub closes the channel (e.g. after unregistering a slow consumer).
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case data, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
