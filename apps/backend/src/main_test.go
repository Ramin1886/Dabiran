package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ramin1886/git-interactive-history/backend/src/api"
	"github.com/ramin1886/git-interactive-history/backend/src/gitengine"
	"github.com/ramin1886/git-interactive-history/backend/src/ws"
)

// newTestMux builds the production routing table over temp storage.
func newTestMux(t *testing.T) *http.ServeMux {
	t.Helper()
	engine := gitengine.NewGitEngine(t.TempDir())
	hub := ws.NewHub()
	go hub.Run()
	return newMux(api.NewAPIServer(engine, nil), hub)
}

func TestHealthEndpoint(t *testing.T) {
	ts := httptest.NewServer(newTestMux(t))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("health request: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || string(body) != "OK" {
		t.Fatalf("health: got %d %q, want 200 OK", resp.StatusCode, body)
	}
}

func TestProtectedRoutesAreRegistered(t *testing.T) {
	ts := httptest.NewServer(newTestMux(t))
	defer ts.Close()

	// Topology must be registered and JWT-protected (401, not 404).
	resp, err := http.Get(ts.URL + "/api/v1/topology?repo_ids=1")
	if err != nil {
		t.Fatalf("topology request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("topology without token: got %d, want 401", resp.StatusCode)
	}
}

func TestWebsocketRoutesAreRegistered(t *testing.T) {
	ts := httptest.NewServer(newTestMux(t))
	defer ts.Close()

	// A plain GET (no upgrade headers) must reach the ws handler and be
	// rejected with 400 Bad Request by the upgrader — 404 would mean the
	// route is missing.
	for _, path := range []string{"/ws", "/ws/repo_map_1"} {
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatalf("ws request %s: %v", path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("%s: got %d, want 400 from upgrader", path, resp.StatusCode)
		}
	}
}
