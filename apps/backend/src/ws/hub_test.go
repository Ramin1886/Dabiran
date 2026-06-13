package ws

import (
	"net/http/httptest"
	"testing"
)

func TestRoomFromRequest(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"/ws/repo_map_1", "repo_map_1"},                 // y-websocket path segment
		{"/ws/repo_map_1/", "repo_map_1"},                // trailing slash tolerated
		{"/ws?room_id=repo_map_2", "repo_map_2"},         // documented query fallback
		{"/ws/repo_map_3?room_id=ignored", "repo_map_3"}, // path wins over query
		{"/ws", "default"},                               // nothing provided
		{"/ws/", "default"},                              // empty path segment
	}
	for _, c := range cases {
		req := httptest.NewRequest("GET", c.url, nil)
		if got := roomFromRequest(req); got != c.want {
			t.Errorf("roomFromRequest(%q) = %q, want %q", c.url, got, c.want)
		}
	}
}

func TestHubRegisterUnregisterCleansRooms(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := &Client{hub: hub, room: "r1", send: make(chan []byte, 1)}
	hub.register <- client
	hub.unregister <- client

	// The send channel must be closed on unregister.
	if _, ok := <-client.send; ok {
		t.Fatal("send channel should be closed after unregister")
	}

	// A duplicate unregister must not panic (double close guard).
	hub.unregister <- client

	// Synchronize: a no-op broadcast proves the loop is still alive.
	hub.broadcast <- frame{room: "r1"}
}
