package api

import (
	"bytes"
	"context"
	"net/http"
	"testing"

	"github.com/ramin1886/git-interactive-history/backend/src/auth"
)

func TestCompactRoomAtomicallyUpdatesDatabase(t *testing.T) {
	pool, srv := depLinksServer(t)
	ts := startTestServer(t, srv)
	token, _ := auth.GenerateToken(auth.DefaultUserID, auth.DefaultTeamID, "Team Owner")

	const room = "test_room_compaction"
	ctx := context.Background()

	// Seed multiple updates for the room
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM yjs_updates WHERE room = $1", room)
	})
	_, err := pool.Exec(ctx, "INSERT INTO yjs_updates (room, update) VALUES ($1, $2), ($1, $3)", room, []byte("update1"), []byte("update2"))
	if err != nil {
		t.Fatalf("failed to seed updates: %v", err)
	}

	// Make the compact request with a new snapshot payload
	snapshot := []byte("compacted_snapshot")
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/rooms/"+room+"/compact", bytes.NewReader(snapshot))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d body=%s", resp.StatusCode, readBody(t, resp))
	}

	// Verify database state: should have exactly 1 update row with the snapshot
	rows, err := pool.Query(ctx, "SELECT update FROM yjs_updates WHERE room = $1 ORDER BY id", room)
	if err != nil {
		t.Fatalf("query updates: %v", err)
	}
	defer rows.Close()

	var count int
	var lastUpdate []byte
	for rows.Next() {
		count++
		if err := rows.Scan(&lastUpdate); err != nil {
			t.Fatalf("scan: %v", err)
		}
	}

	if count != 1 {
		t.Fatalf("expected exactly 1 update row, got %d", count)
	}
	if !bytes.Equal(lastUpdate, snapshot) {
		t.Fatalf("expected update to match snapshot, got %q", lastUpdate)
	}
}

func TestCompactRoomRequiresAuth(t *testing.T) {
	_, srv := depLinksServer(t)
	ts := startTestServer(t, srv)

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/rooms/test_room/compact", bytes.NewReader([]byte("data")))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}
