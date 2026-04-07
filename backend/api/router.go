package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-git/go-git/v5"
	"github.com/ramin1886/git-interactive-history/backend/gitengine"
)

type APIServer struct {
	Engine *gitengine.GitEngine
}

func NewAPIServer(engine *gitengine.GitEngine) *APIServer {
	return &APIServer{Engine: engine}
}

// ServeTopology returns a generated commit graph JSON given an active repository.
func (s *APIServer) ServeTopology(w http.ResponseWriter, r *http.Request) {
	// In a real flow: extract Repo ID from path, Auth from token, map it to local Engine.
	// We will simulate the interaction here for architecture setup.

	// Placeholder hardcoded interaction against bare repo if existed
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

// AddRoutes mounts API logic onto the global multiplexer
func (s *APIServer) AddRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/topology", s.ServeTopology)
}
