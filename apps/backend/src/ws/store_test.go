package ws

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// liveStore connects to DATABASE_URL, ensures the yjs_updates table exists,
// and returns a pgStore plus a cleanup that deletes the test room's rows. It
// skips the test when no database is configured.
func liveStore(t *testing.T, room string) (*pgStore, func()) {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set; skipping live store test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	// Minimal idempotent DDL so the test is self-contained even if Migrate
	// has not run against this database yet.
	if _, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS yjs_updates (
		id SERIAL PRIMARY KEY, room TEXT NOT NULL, update BYTEA NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		pool.Close()
		t.Fatalf("ensure table: %v", err)
	}
	cleanup := func() {
		pool.Exec(context.Background(), "DELETE FROM yjs_updates WHERE room = $1", room)
		pool.Close()
	}
	return &pgStore{pool: pool}, cleanup
}

// uniqueRoom returns a per-test room name so concurrent runs never collide.
func uniqueRoom(prefix string) string {
	return fmt.Sprintf("test_%s_%d", prefix, time.Now().UnixNano())
}

func TestNoopStore(t *testing.T) {
	s := NewStore(nil)
	if _, ok := s.(noopStore); !ok {
		t.Fatalf("nil pool should yield noopStore, got %T", s)
	}
	if err := s.Append(context.Background(), "r", []byte("x")); err != nil {
		t.Fatalf("noop Append: %v", err)
	}
	got, err := s.Load(context.Background(), "r")
	if err != nil || got != nil {
		t.Fatalf("noop Load: got %v, %v", got, err)
	}
}

func TestPgStoreAppendLoadOrdering(t *testing.T) {
	room := uniqueRoom("append")
	s, cleanup := liveStore(t, room)
	defer cleanup()
	ctx := context.Background()

	updates := [][]byte{[]byte("first"), []byte("second"), []byte("third")}
	for _, u := range updates {
		if err := s.Append(ctx, room, u); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}

	got, err := s.Load(ctx, room)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != len(updates) {
		t.Fatalf("expected %d updates, got %d", len(updates), len(got))
	}
	for i, want := range updates {
		if !bytes.Equal(got[i], want) {
			t.Fatalf("update %d: got %q want %q (ordering broken)", i, got[i], want)
		}
	}
}

func TestPgStoreLoadEmptyRoom(t *testing.T) {
	room := uniqueRoom("empty")
	s, cleanup := liveStore(t, room)
	defer cleanup()
	got, err := s.Load(context.Background(), room)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("empty room should have no updates, got %d", len(got))
	}
}
