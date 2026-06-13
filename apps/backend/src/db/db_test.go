package db

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestConnectUnreachableDatabase(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Port 1 is essentially guaranteed to refuse connections.
	if _, err := Connect(ctx, "postgres://nobody:nothing@127.0.0.1:1/nodb?sslmode=disable"); err == nil {
		t.Fatal("Connect to unreachable database should fail")
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set; skipping live database migration test")
	}
	ctx := context.Background()

	pool, err := Connect(ctx, url)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer pool.Close()

	// Running the migration twice must succeed (CREATE TABLE IF NOT EXISTS).
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}

	// All four tables must exist.
	for _, table := range []string{"users", "teams", "repositories", "annotations"} {
		var exists bool
		err := pool.QueryRow(ctx,
			"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)",
			table).Scan(&exists)
		if err != nil || !exists {
			t.Fatalf("table %s missing after Migrate (err=%v)", table, err)
		}
	}
}
