package ws

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store is the durable backing for Yjs collaboration rooms.
//
// PERSISTENCE DESIGN (append-only Yjs-update log): the relay does not parse
// the Yjs binary protocol, so it cannot merge updates into a single document
// state in Go. Instead it persists every inbound update verbatim, in arrival
// order, and replays the whole ordered log to a lone joining client. The Yjs
// client merges the replayed updates (each delivered as its own binary frame)
// into its local document, reconstructing the full prior state — so a
// re-joining client sees previously-drawn annotations. This is correct because
// Yjs updates are commutative and idempotent when applied as updates.
type Store interface {
	// Append durably records one inbound Yjs update for room, preserving
	// arrival order.
	Append(ctx context.Context, room string, update []byte) error
	// Load returns all stored updates for room in arrival (insertion) order.
	Load(ctx context.Context, room string) ([][]byte, error)
}

// noopStore is used when no database is configured. It persists nothing and
// replays nothing, preserving the original "dumb relay" behaviour.
type noopStore struct{}

// Append discards the update. GoDoc: no-op (no DB configured).
func (noopStore) Append(context.Context, string, []byte) error { return nil }

// Load returns no prior state. GoDoc: no-op (no DB configured).
func (noopStore) Load(context.Context, string) ([][]byte, error) { return nil, nil }

// pgStore persists Yjs updates in the append-only yjs_updates table.
type pgStore struct{ pool *pgxpool.Pool }

// NewStore returns a Postgres-backed Store when pool is non-nil, otherwise a
// no-op Store that keeps the relay working without persistence.
func NewStore(pool *pgxpool.Pool) Store {
	if pool == nil {
		return noopStore{}
	}
	return &pgStore{pool: pool}
}

// Append inserts one update row for room. The SERIAL id preserves arrival
// order for replay.
func (s *pgStore) Append(ctx context.Context, room string, update []byte) error {
	_, err := s.pool.Exec(ctx,
		"INSERT INTO yjs_updates (room, update) VALUES ($1, $2)", room, update)
	return err
}

// Load reads every stored update for room ordered by ascending id (arrival
// order). It returns an empty slice when the room has no history.
func (s *pgStore) Load(ctx context.Context, room string) ([][]byte, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT update FROM yjs_updates WHERE room = $1 ORDER BY id", room)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	updates := make([][]byte, 0)
	for rows.Next() {
		var u []byte
		if err := rows.Scan(&u); err != nil {
			return nil, err
		}
		updates = append(updates, u)
	}
	return updates, rows.Err()
}
