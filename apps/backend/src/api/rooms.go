package api

import (
	"io"
	"net/http"
	"strings"
)

// CompactRoom handles POST /api/v1/rooms/{room}/compact (JWT required).
// It deletes all prior Yjs updates stored in the database for the given room
// and replaces them with a single compacted update (the full document state)
// sent as the binary body. This execution runs atomically inside a database
// transaction to prevent data loss or service disruption.
// Requires the database pool; returns 503 when s.DB is nil.
func (s *APIServer) CompactRoom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	// Parse room from path: /api/v1/rooms/{room}/compact
	path := r.URL.Path
	const prefix = "/api/v1/rooms/"
	const suffix = "/compact"
	idx := strings.Index(path, prefix)
	sfxIdx := strings.LastIndex(path, suffix)
	if idx == -1 || sfxIdx == -1 || sfxIdx <= idx+len(prefix) {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	room := path[idx+len(prefix) : sfxIdx]
	if room == "" {
		http.Error(w, "room name is required", http.StatusBadRequest)
		return
	}

	// Read binary update from body, limited to 10MB to support large documents.
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 10<<20))
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	// Begin atomic transaction
	tx, err := s.DB.Begin(r.Context())
	if err != nil {
		http.Error(w, "failed to start transaction", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(r.Context())

	// 1. Delete all updates for the room
	_, err = tx.Exec(r.Context(), "DELETE FROM yjs_updates WHERE room = $1", room)
	if err != nil {
		http.Error(w, "failed to delete updates", http.StatusInternalServerError)
		return
	}

	// 2. Insert the single compacted snapshot
	_, err = tx.Exec(r.Context(), "INSERT INTO yjs_updates (room, update) VALUES ($1, $2)", room, body)
	if err != nil {
		http.Error(w, "failed to insert compacted update", http.StatusInternalServerError)
		return
	}

	// Commit transaction
	if err := tx.Commit(r.Context()); err != nil {
		http.Error(w, "failed to commit transaction", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
