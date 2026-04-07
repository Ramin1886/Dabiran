package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ramin1886/git-interactive-history/backend/api"
	"github.com/ramin1886/git-interactive-history/backend/gitengine"
	"github.com/ramin1886/git-interactive-history/backend/ws"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://git_viz:secret_password@localhost:5432/git_interactive_history?sslmode=disable"
	}

	_, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to create connection pool: %v\n", err)
	} else {
		fmt.Println("Connected to PostgreSQL successfully.")
	}

	// Initialize Git Engine layer
	cwd, _ := os.Getwd()
	repoCachePath := filepath.Join(cwd, "repos")
	engine := gitengine.NewGitEngine(repoCachePath)
	
	// Initialize WebSocket CRDT Relay Hub
	hub := ws.NewHub()
	go hub.Run()

	// Mount API
	apiServer := api.NewAPIServer(engine)
	mux := http.NewServeMux()
	
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	
	apiServer.AddRoutes(mux)
	
	// Mount WebSocket
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		ws.ServeWs(hub, w, r)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting server on port %s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
