// Command main boots the Git Interactive History backend: PostgreSQL
// (optional), the git engine cache, the REST API, and the websocket relay.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ramin1886/git-interactive-history/backend/src/api"
	"github.com/ramin1886/git-interactive-history/backend/src/auth"
	"github.com/ramin1886/git-interactive-history/backend/src/db"
	"github.com/ramin1886/git-interactive-history/backend/src/gitengine"
	"github.com/ramin1886/git-interactive-history/backend/src/ws"
)

// connectDatabase connects to dbURL and runs the idempotent migrations. It
// returns nil (and logs a warning) when the database is unreachable so local
// development without Postgres still serves filesystem-backed topology.
func connectDatabase(ctx context.Context, dbURL string) *pgxpool.Pool {
	pool, err := db.Connect(ctx, dbURL)
	if err != nil {
		log.Printf("WARNING: database unreachable, continuing without persistence: %v", err)
		return nil
	}
	if err := db.Migrate(ctx, pool); err != nil {
		log.Printf("WARNING: schema migration failed, continuing without persistence: %v", err)
		pool.Close()
		return nil
	}
	// Seed the default identity backing the dev LoginMock so the repository
	// API works against a fresh database without manual SQL.
	if err := db.SeedSingleTenant(ctx, pool, auth.DefaultUserID, auth.DefaultTeamID); err != nil {
		log.Printf("WARNING: failed to seed single-tenant identity: %v", err)
	}
	log.Println("Connected to PostgreSQL and applied schema.")
	return pool
}

// newMux assembles the HTTP routing table: the /health probe, the REST API
// routes, and the websocket relay on both /ws (?room_id= fallback) and
// /ws/<room> (y-websocket path-segment convention).
func newMux(apiServer *api.APIServer, hub *ws.Hub) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	apiServer.AddRoutes(mux)
	wsHandler := func(w http.ResponseWriter, r *http.Request) { ws.ServeWs(hub, w, r) }
	mux.HandleFunc("/ws", wsHandler)
	mux.HandleFunc("/ws/", wsHandler)
	return mux
}

// withCORS wraps h with permissive CORS headers so the browser frontend
// (served from a different origin in development, e.g. http://localhost:3000)
// can call the API with an Authorization header. Preflight OPTIONS requests
// are answered immediately with 204. In production, front the API with a
// reverse proxy and tighten the allowed origin.
func withCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// main wires configuration from the environment, starts the websocket hub,
// and serves HTTP on $PORT (default 8080).
func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://git_viz:secret_password@localhost:5432/git_interactive_history?sslmode=disable"
	}
	pool := connectDatabase(context.Background(), dbURL)
	if pool != nil {
		defer pool.Close()
	}

	cwd, _ := os.Getwd()
	engine := gitengine.NewGitEngine(filepath.Join(cwd, "repos"))

	// Wire the db pool into the relay so Yjs documents persist and replay to
	// lone re-joining clients; with a nil pool the hub uses a no-op store.
	hub := ws.NewHubWithStore(pool)
	go hub.Run()

	mux := newMux(api.NewAPIServer(engine, pool), hub)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Starting server on port %s", port)
	if err := http.ListenAndServe(":"+port, withCORS(mux)); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
