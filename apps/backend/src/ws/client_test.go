package ws

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// dialRoom connects a websocket client to the test server in the given room
// (using the /ws/<room> path convention) and registers cleanup.
func dialRoom(t *testing.T, ts *httptest.Server, room string) *websocket.Conn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/" + room
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial %s: %v", url, err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// newRelayServer boots a hub plus an httptest server exposing ServeWs.
func newRelayServer(t *testing.T) *httptest.Server {
	t.Helper()
	hub := NewHub()
	go hub.Run()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ServeWs(hub, w, r)
	}))
	t.Cleanup(ts.Close)
	return ts
}

// expectNoMessage asserts that no frame arrives on conn within wait.
func expectNoMessage(t *testing.T, conn *websocket.Conn, wait time.Duration) {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(wait))
	if _, data, err := conn.ReadMessage(); err == nil {
		t.Fatalf("unexpected message received: %v", data)
	}
}

func TestRelayBroadcastsToRoomButNotSender(t *testing.T) {
	ts := newRelayServer(t)
	connA := dialRoom(t, ts, "room1")
	connB := dialRoom(t, ts, "room1")
	connC := dialRoom(t, ts, "room2") // different room: must stay silent

	// Give the hub a moment to process all registrations.
	time.Sleep(100 * time.Millisecond)

	payload := []byte{0x01, 0x02, 0x03, 0xFF}
	if err := connA.WriteMessage(websocket.BinaryMessage, payload); err != nil {
		t.Fatalf("write from A: %v", err)
	}

	connB.SetReadDeadline(time.Now().Add(2 * time.Second))
	msgType, data, err := connB.ReadMessage()
	if err != nil {
		t.Fatalf("B did not receive relayed frame: %v", err)
	}
	if msgType != websocket.BinaryMessage || !bytes.Equal(data, payload) {
		t.Fatalf("B got type=%d data=%v, want binary %v", msgType, data, payload)
	}

	// The frame must not be echoed back to the sender...
	expectNoMessage(t, connA, 300*time.Millisecond)
	// ...and must not leak into other rooms.
	expectNoMessage(t, connC, 300*time.Millisecond)
}

func TestRelayQueryParamRoomFallback(t *testing.T) {
	ts := newRelayServer(t)
	wsBase := "ws" + strings.TrimPrefix(ts.URL, "http")

	// One client joins via path segment, the other via ?room_id= — they must
	// land in the same room.
	connA := dialRoom(t, ts, "shared")
	connB, _, err := websocket.DefaultDialer.Dial(wsBase+"/ws?room_id=shared", nil)
	if err != nil {
		t.Fatalf("dial with room_id: %v", err)
	}
	t.Cleanup(func() { connB.Close() })

	time.Sleep(100 * time.Millisecond)

	payload := []byte("yjs-update")
	if err := connA.WriteMessage(websocket.BinaryMessage, payload); err != nil {
		t.Fatalf("write: %v", err)
	}
	connB.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := connB.ReadMessage()
	if err != nil {
		t.Fatalf("query-param client missed the frame: %v", err)
	}
	if !bytes.Equal(data, payload) {
		t.Fatalf("got %q want %q", data, payload)
	}
}

func TestRelayDisconnectDoesNotBreakRoom(t *testing.T) {
	ts := newRelayServer(t)
	connA := dialRoom(t, ts, "roomX")
	connB := dialRoom(t, ts, "roomX")
	connGone := dialRoom(t, ts, "roomX")

	time.Sleep(100 * time.Millisecond)
	connGone.Close()
	time.Sleep(100 * time.Millisecond)

	payload := []byte{0xAA}
	if err := connA.WriteMessage(websocket.BinaryMessage, payload); err != nil {
		t.Fatalf("write after peer disconnect: %v", err)
	}
	connB.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := connB.ReadMessage()
	if err != nil {
		t.Fatalf("B missed frame after peer disconnect: %v", err)
	}
	if !bytes.Equal(data, payload) {
		t.Fatalf("got %v want %v", data, payload)
	}
}
