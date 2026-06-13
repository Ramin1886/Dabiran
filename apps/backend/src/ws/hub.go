// Package ws implements a room-scoped binary relay for Yjs/y-websocket clients
// with optional durable document persistence.
//
// RELAY: the hub does not interpret the Yjs sync protocol. Every binary frame
// received from a client is forwarded verbatim to all OTHER clients in the
// same room; the peers themselves answer Yjs sync steps (SyncStep1/2).
//
// PERSISTENCE: when a Store is wired in (Postgres-backed), the hub also
// persists each inbound update to an append-only log and, when a client joins
// an empty room, replays the stored updates to that single client before live
// traffic — so a lone re-joining client recovers previously-drawn annotations.
// With the no-op Store (no database) the hub keeps its original "dumb relay"
// behaviour: a lone client receives no initial state. See store.go for the
// append-only-log design rationale.
package ws

import (
	"context"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
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
	// persistCh feeds the single persistence worker so updates are appended in
	// the exact order the hub broadcast them (replay must preserve order).
	persistCh chan frame
	// store durably persists and replays Yjs updates. It is never nil:
	// NewHub installs a no-op store when no pool is supplied.
	store Store
}

// NewHub allocates an idle Hub with no document persistence (a no-op store).
// Call Run (usually in a goroutine) to start processing registrations and
// broadcasts. Use NewHubWithStore to enable durable persistence.
func NewHub() *Hub {
	return NewHubWithStore(nil)
}

// NewHubWithStore allocates an idle Hub backed by a Postgres update log when
// pool is non-nil, or the no-op store (original relay-only behaviour) when it
// is nil. main.go wires the application's db pool here.
func NewHubWithStore(pool *pgxpool.Pool) *Hub {
	return &Hub{
		rooms:      make(map[string]map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan frame),
		persistCh:  make(chan frame, 256),
		store:      NewStore(pool),
	}
}

// Run executes the hub event loop: it adds and removes clients from rooms and
// fans each frame out to every room member except the sender. It blocks
// forever and is intended to run in its own goroutine.
func (h *Hub) Run() {
	go h.persistLoop()
	for {
		select {
		case c := <-h.register:
			members, ok := h.rooms[c.room]
			if !ok {
				members = make(map[*Client]bool)
				h.rooms[c.room] = members
			}
			firstInRoom := len(members) == 0
			members[c] = true
			if firstInRoom {
				// First client in the room: replay any persisted history so a
				// lone joiner recovers prior state. Done off the hub loop so
				// store I/O never blocks broadcasts; updates are queued onto
				// the client's buffered send channel ahead of live traffic.
				go h.replayHistory(c)
			}
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
			// Hand the update to the single persistence worker so appends keep
			// hub-broadcast order. f.sender is nil for internal synchronization
			// frames (tests); skip persisting those. Non-blocking: if the
			// worker is saturated, drop persistence rather than stall the hub
			// (Yjs sync still reconciles peers live).
			if f.sender != nil && len(f.data) > 0 {
				dup := make([]byte, len(f.data))
				copy(dup, f.data)
				select {
				case h.persistCh <- frame{room: f.room, data: dup}:
				default:
					log.Printf("ws: persist queue full, dropping update for room %q", f.room)
				}
			}
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

// replayHistory loads the persisted update log for c's room and queues each
// stored update onto c's send channel as a binary frame, in arrival order,
// before live traffic. It degrades gracefully: a store error is logged and the
// client simply starts with no history (relay-only behaviour). It runs in its
// own goroutine off the hub loop.
func (h *Hub) replayHistory(c *Client) {
	updates, err := h.store.Load(context.Background(), c.room)
	if err != nil {
		log.Printf("ws: replay history for room %q failed, continuing without prior state: %v", c.room, err)
		return
	}
	for _, u := range updates {
		select {
		case c.send <- u:
		default:
			// Backpressure on the joiner: drop the rest rather than block.
			// Live Yjs sync will still reconcile via SyncStep1/2 once peers join.
			log.Printf("ws: replay buffer full for room %q, truncating history", c.room)
			return
		}
	}
}

// persistLoop is the single worker draining persistCh, appending each update
// to the durable log in the exact order the hub queued it (so replay preserves
// arrival order). Running off the hub loop keeps store I/O from blocking
// broadcasts. Errors are logged and swallowed so persistence never breaks the
// relay. It blocks forever and runs in its own goroutine.
func (h *Hub) persistLoop() {
	for f := range h.persistCh {
		if err := h.store.Append(context.Background(), f.room, f.data); err != nil {
			log.Printf("ws: persist update for room %q failed: %v", f.room, err)
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
