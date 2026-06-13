// Package db owns PostgreSQL connectivity and the idempotent schema
// migration for the persistence layer (users, teams, team_memberships,
// repositories, annotations — see src/models/models.go).
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect opens a pgx connection pool against url and verifies liveness with
// a 5-second ping. It returns the ready pool, or an error if the database is
// unreachable (in which case no pool is left open).
func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// schemaDDL creates the relational schema matching src/models/models.go.
// Every statement is CREATE TABLE IF NOT EXISTS so Migrate can run on every
// boot. Foreign keys enforce the documented Team-to-Repo 1:N isolation
// (docs/architecture.md "Multi-Tenant Identity Control").
const schemaDDL = `
CREATE TABLE IF NOT EXISTS users (
	id         SERIAL PRIMARY KEY,
	email      TEXT NOT NULL UNIQUE,
	name       TEXT NOT NULL DEFAULT '',
	role       TEXT NOT NULL DEFAULT 'Team Member',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS teams (
	id         SERIAL PRIMARY KEY,
	name       TEXT NOT NULL,
	owner_id   INTEGER NOT NULL REFERENCES users(id),
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS repositories (
	id                   SERIAL PRIMARY KEY,
	team_id              INTEGER NOT NULL REFERENCES teams(id),
	name                 TEXT NOT NULL,
	url                  TEXT NOT NULL,
	auth_type            TEXT NOT NULL DEFAULT '',
	encrypted_credential TEXT NOT NULL DEFAULT '',
	created_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS team_memberships (
	id      SERIAL PRIMARY KEY,
	team_id INTEGER NOT NULL REFERENCES teams(id),
	user_id INTEGER NOT NULL REFERENCES users(id),
	role    TEXT NOT NULL DEFAULT 'Team Member',
	UNIQUE(team_id, user_id)
);

CREATE TABLE IF NOT EXISTS annotations (
	id            SERIAL PRIMARY KEY,
	repository_id INTEGER NOT NULL REFERENCES repositories(id),
	type          TEXT NOT NULL,
	payload       TEXT NOT NULL DEFAULT '',
	author_id     INTEGER NOT NULL REFERENCES users(id),
	created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Idempotent column add for repositories tables created before auth_type
-- existed (CREATE TABLE IF NOT EXISTS does not retro-fit columns).
ALTER TABLE repositories ADD COLUMN IF NOT EXISTS auth_type TEXT NOT NULL DEFAULT '';

-- Append-only log of raw Yjs update bytes per collaboration room. The ws
-- relay inserts one row per inbound binary frame and replays them in id order
-- to a lone joining client so it sees previously-drawn annotations
-- (apps/backend/src/ws/store.go).
CREATE TABLE IF NOT EXISTS yjs_updates (
	id         SERIAL PRIMARY KEY,
	room       TEXT NOT NULL,
	update     BYTEA NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_yjs_updates_room ON yjs_updates (room, id);

-- Per-user named snapshots of the frontend canvas view state (viewport +
-- active filters). state is an opaque, frontend-owned JSON string the backend
-- never parses (apps/backend/src/api/views.go).
CREATE TABLE IF NOT EXISTS canvas_views (
	id         SERIAL PRIMARY KEY,
	user_id    INTEGER NOT NULL REFERENCES users(id),
	team_id    INTEGER NOT NULL REFERENCES teams(id),
	name       TEXT NOT NULL,
	state      TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
`

// Migrate applies the idempotent schema DDL on pool. Safe to call on every
// startup; returns the first execution error, if any.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, schemaDDL); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	return nil
}

// SeedSingleTenant ensures the single-tenant default identity backing the
// development LoginMock exists: a user with id=userID and a team with
// id=teamID owned by that user, plus their membership. Without these rows the
// repositories foreign keys (team_id -> teams, owner_id -> users) reject any
// write made under the mock identity. It is idempotent (ON CONFLICT DO
// NOTHING) and realigns the SERIAL sequences afterwards so subsequent
// auto-increment inserts do not collide with the explicitly seeded ids.
//
// The real GitHub OAuth flow creates its own users/teams, so these seed rows
// are harmless in production; they only make local dev work out of the box.
func SeedSingleTenant(ctx context.Context, pool *pgxpool.Pool, userID, teamID int) error {
	// pgx cannot run multiple parameterized commands in one Exec, so each
	// statement is issued separately.
	stmts := []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO users (id, email, name, role) VALUES ($1, 'dev@localhost', 'Development User', 'Team Owner') ON CONFLICT (id) DO NOTHING`, []any{userID}},
		{`INSERT INTO teams (id, name, owner_id) VALUES ($1, 'Default Workspace', $2) ON CONFLICT (id) DO NOTHING`, []any{teamID, userID}},
		{`INSERT INTO team_memberships (team_id, user_id, role) VALUES ($1, $2, 'Team Owner') ON CONFLICT (team_id, user_id) DO NOTHING`, []any{teamID, userID}},
		// Realign SERIAL sequences so future auto-increment inserts do not
		// collide with the explicitly seeded ids.
		{`SELECT setval(pg_get_serial_sequence('users', 'id'), GREATEST((SELECT MAX(id) FROM users), 1))`, nil},
		{`SELECT setval(pg_get_serial_sequence('teams', 'id'), GREATEST((SELECT MAX(id) FROM teams), 1))`, nil},
	}
	for _, s := range stmts {
		if _, err := pool.Exec(ctx, s.sql, s.args...); err != nil {
			return fmt.Errorf("seed single-tenant identity: %w", err)
		}
	}
	return nil
}
