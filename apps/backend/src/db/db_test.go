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

func TestSeedSingleTenantIsIdempotent(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set; skipping live seed test")
	}
	ctx := context.Background()

	pool, err := Connect(ctx, url)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer pool.Close()

	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// Seeding twice must succeed and leave exactly one user/team for the ids.
	const userID, teamID = 1, 100
	if err := SeedSingleTenant(ctx, pool, userID, teamID); err != nil {
		t.Fatalf("first SeedSingleTenant: %v", err)
	}
	if err := SeedSingleTenant(ctx, pool, userID, teamID); err != nil {
		t.Fatalf("second SeedSingleTenant: %v", err)
	}

	var users, teams int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM users WHERE id=$1", userID).Scan(&users); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM teams WHERE id=$1", teamID).Scan(&teams); err != nil {
		t.Fatalf("count teams: %v", err)
	}
	if users != 1 || teams != 1 {
		t.Fatalf("expected exactly one seeded user and team, got users=%d teams=%d", users, teams)
	}

	// The seeded team must satisfy the repositories foreign key: an insert
	// under the default identity must now succeed (and be cleaned up).
	var repoID int
	err = pool.QueryRow(ctx,
		"INSERT INTO repositories (team_id, name, url) VALUES ($1, 'seed-test', 'x') RETURNING id",
		teamID).Scan(&repoID)
	if err != nil {
		t.Fatalf("insert repository under seeded team should succeed: %v", err)
	}
	if _, err := pool.Exec(ctx, "DELETE FROM repositories WHERE id=$1", repoID); err != nil {
		t.Fatalf("cleanup repository: %v", err)
	}
}
