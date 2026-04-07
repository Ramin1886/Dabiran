package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-git/go-git/v5"
	"github.com/ramin1886/git-interactive-history/backend/gitengine"
)

// APIServer binds the core logic engine to stateless HTTP endpoints.
type APIServer struct {
	Engine *gitengine.GitEngine
}

// NewAPIServer creates the route bounds for the topology handler.
func NewAPIServer(engine *gitengine.GitEngine) *APIServer {
	return &APIServer{Engine: engine}
}

// ServeTopology responds with chronologically sequenced graphical commits maps.
func (s *APIServer) ServeTopology(w http.ResponseWriter, r *http.Request) {
	repoPath := r.URL.Query().Get("repo_path")
	if repoPath == "" {
		http.Error(w, "missing repo_path", http.StatusBadRequest)
		return
	}

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		http.Error(w, "failed to open bare repo. Sync required.", http.StatusNotFound)
		return
	}

	nodes, err := gitengine.ExtractTopology(repo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(nodes)
}

// AddRoutes exposes explicit REST boundaries to MUX.
func (s *APIServer) AddRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/topology", s.ServeTopology)
}
