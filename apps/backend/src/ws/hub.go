// Package ws implements a room-scoped binary relay for Yjs/y-websocket clients.
//
// LIMITATION (by design, "dumb relay"): the hub does not interpret the Yjs
// sync protocol and keeps no server-side copy of the document. Every binary
// frame received from a client is forwarded verbatim to all OTHER clients in
// the same room; the peers themselves answer Yjs sync steps (SyncStep1/2).
// Consequently a client that is alone in a room receives no initial state —
// document persistence/snapshotting would be required to change that.
package ws

import (
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

// frame is a single message queued for relay to a room.
type frame struct {
	room   string
	sender *Client
	data   []byte
}

// Hub routes binary frames between clients grouped by room. All membership
// mutations and broadcasts are serialized through Run's event loop, so no
// additional locking is needed.
type Hub struct {
	rooms      map[string]map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan frame
}

// NewHub allocates an idle Hub. Call Run (usually in a goroutine) to start
// processing registrations and broadcasts.
func NewHub() *Hub {
	return &Hub{
		rooms:      make(map[string]map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan frame),
	}
}

// Run executes the hub event loop: it adds and removes clients from rooms and
// fans each frame out to every room member except the sender. It blocks
// forever and is intended to run in its own goroutine.
func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			members, ok := h.rooms[c.room]
			if !ok {
				members = make(map[*Client]bool)
				h.rooms[c.room] = members
			}
			members[c] = true
		case c := <-h.unregister:
			if members, ok := h.rooms[c.room]; ok {
				if members[c] {
					delete(members, c)
					close(c.send)
					if len(members) == 0 {
						delete(h.rooms, c.room)
					}
				}
			}
		case f := <-h.broadcast:
			for c := range h.rooms[f.room] {
				if c == f.sender {
					continue
				}
				select {
				case c.send <- f.data:
				default:
					// Slow consumer: drop it rather than blocking the hub.
					delete(h.rooms[f.room], c)
					close(c.send)
				}
			}
		}
	}
}

// roomFromRequest resolves the relay room for an upgrade request. Primary
// contract (matches the y-websocket client): the room is the path segment
// after "/ws/", e.g. ws://host/ws/repo_map_1. Fallback (documented in
// docs/apis_doc.md): the ?room_id= query parameter. Defaults to "default"
// when neither is present.
func roomFromRequest(r *http.Request) string {
	if idx := strings.Index(r.URL.Path, "/ws/"); idx != -1 {
		if room := strings.Trim(r.URL.Path[idx+len("/ws/"):], "/"); room != "" {
			return room
		}
	}
	if room := r.URL.Query().Get("room_id"); room != "" {
		return room
	}
	return "default"
}

// upgrader performs the HTTP -> WebSocket handshake. CheckOrigin accepts all
// origins because the relay carries no ambient authority (auth happens on the
// REST API); tighten this when cookies-based auth ever reaches this endpoint.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// ServeWs upgrades an HTTP request to a WebSocket connection, joins the
// client to its room on hub, and starts its read and write pumps. It writes
// an HTTP error itself if the upgrade fails.
func ServeWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		// Upgrade already replied with an HTTP error.
		return
	}
	client := &Client{
		hub:  hub,
		conn: conn,
		room: roomFromRequest(r),
		send: make(chan []byte, 256),
	}
	hub.register <- client
	go client.writePump()
	go client.readPump()
}
