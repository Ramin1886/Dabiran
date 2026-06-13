package ws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
)

// dialWS opens a websocket client against the test server's /ws/<room> path.
func dialWS(t *testing.T, serverURL, room string) *websocket.Conn {
	t.Helper()
	u := "ws" + strings.TrimPrefix(serverURL, "http") + "/ws/" + room
	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		t.Fatalf("dial %s: %v", u, err)
	}
	return conn
}

// readBinary reads one binary frame with a deadline, failing the test on
// timeout or non-binary frames.
func readBinary(t *testing.T, conn *websocket.Conn) []byte {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	mt, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if mt != websocket.BinaryMessage {
		t.Fatalf("expected binary frame, got message type %d", mt)
	}
	return data
}

// TestLateJoinerReceivesPersistedUpdates is the core acceptance test for
// Feature A: a client that joins a room AFTER all prior clients have left must
// still receive the previously-sent updates, replayed from the store.
func TestLateJoinerReceivesPersistedUpdates(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set; skipping ws persistence integration test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS yjs_updates (
		id SERIAL PRIMARY KEY, room TEXT NOT NULL, update BYTEA NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		t.Fatalf("ensure table: %v", err)
	}

	room := uniqueRoom("relay")
	defer pool.Exec(context.Background(), "DELETE FROM yjs_updates WHERE room = $1", room)

	hub := NewHubWithStore(pool)
	go hub.Run()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ServeWs(hub, w, r)
	}))
	defer srv.Close()

	// First client joins and sends two updates, which get persisted.
	c1 := dialWS(t, srv.URL, room)
	update1 := []byte("annotation-update-1")
	update2 := []byte("annotation-update-2")
	if err := c1.WriteMessage(websocket.BinaryMessage, update1); err != nil {
		t.Fatalf("write update1: %v", err)
	}
	if err := c1.WriteMessage(websocket.BinaryMessage, update2); err != nil {
		t.Fatalf("write update2: %v", err)
	}

	// Wait for the async persistence to land (poll the store directly).
	store := &pgStore{pool: pool}
	deadline := time.Now().Add(5 * time.Second)
	for {
		got, lerr := store.Load(ctx, room)
		if lerr != nil {
			t.Fatalf("Load: %v", lerr)
		}
		if len(got) >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("updates not persisted in time, got %d", len(got))
		}
		time.Sleep(50 * time.Millisecond)
	}

	// First client leaves: the room becomes empty.
	c1.Close()
	time.Sleep(100 * time.Millisecond)

	// A lone late joiner must receive both prior updates as binary frames,
	// in order, before any live traffic.
	c2 := dialWS(t, srv.URL, room)
	defer c2.Close()

	first := readBinary(t, c2)
	second := readBinary(t, c2)
	if string(first) != string(update1) || string(second) != string(update2) {
		t.Fatalf("late joiner replay mismatch: got %q,%q want %q,%q",
			first, second, update1, update2)
	}
}

// TestNoStoreNoReplay verifies the relay keeps its original behaviour with no
// database: a lone joiner receives no initial state.
func TestNoStoreNoReplay(t *testing.T) {
	hub := NewHub() // no-op store
	go hub.Run()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ServeWs(hub, w, r)
	}))
	defer srv.Close()

	conn := dialWS(t, srv.URL, uniqueRoom("nostore"))
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatal("lone joiner should receive no initial state without a store")
	}
}
