// Package api exposes the REST surface documented in docs/apis_doc.md:
// authentication endpoints and the unified topology extractor.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ramin1886/git-interactive-history/backend/src/auth"
	"github.com/ramin1886/git-interactive-history/backend/src/gitengine"
)

// APIServer binds the git engine and (optionally) the database pool to the
// stateless HTTP handlers.
type APIServer struct {
	Engine *gitengine.GitEngine
	// DB is optional: when nil (e.g. local dev without Postgres) repository
	// lookups fall back to bare repos under Engine.BaseStoragePath.
	DB *pgxpool.Pool
}

// NewAPIServer constructs an APIServer over engine. pool may be nil to run
// without database-backed repository metadata.
func NewAPIServer(engine *gitengine.GitEngine, pool *pgxpool.Pool) *APIServer {
	return &APIServer{Engine: engine, DB: pool}
}

// LoginMock issues a development JWT for the single-tenant default identity
// (POST/GET /api/v1/auth/login). Response: {"access_token", "role"}.
func (s *APIServer) LoginMock(w http.ResponseWriter, r *http.Request) {
	const role = "Team Owner"
	token, err := auth.GenerateToken(auth.DefaultUserID, auth.DefaultTeamID, role)
	if err != nil {
		http.Error(w, "failed to issue token", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]string{"access_token": token, "role": role})
}

// openRepository resolves one repo id to a git repository. If a repositories
// row exists in the database, the repo is cloned/refreshed from its URL via
// the engine; otherwise it falls back to a local bare repo at
// <BaseStoragePath>/mock_<id>.git, then <BaseStoragePath>/repo_<id>.git.
func (s *APIServer) openRepository(ctx context.Context, id string) (*git.Repository, error) {
	if s.DB != nil {
		if numericID, err := strconv.Atoi(id); err == nil {
			var url string
			row := s.DB.QueryRow(ctx, "SELECT url FROM repositories WHERE id = $1", numericID)
			if err := row.Scan(&url); err == nil {
				// TODO: decrypt EncryptedCredential via the secrets layer and
				// pass real auth; anonymous fetch covers public repos for now.
				return s.Engine.EnsureRepository(ctx, numericID, url, "", "")
			}
		}
	}
	for _, name := range []string{fmt.Sprintf("mock_%s.git", id), fmt.Sprintf("repo_%s.git", id)} {
		if repo, err := git.PlainOpen(filepath.Join(s.Engine.BaseStoragePath, name)); err == nil {
			return repo, nil
		}
	}
	return nil, fmt.Errorf("repository %q not found", id)
}

// ServeTopology handles GET /api/v1/topology?repo_ids=1,2 — it resolves each
// repository, extracts the unified chronological topology, and writes the
// CommitNode JSON array. Requires a valid JWT (enforced by RequireAuth);
// callers outside the single-tenant default team get 403.
func (s *APIServer) ServeTopology(w http.ResponseWriter, r *http.Request) {
	repoIDsParam := r.URL.Query().Get("repo_ids")
	if repoIDsParam == "" {
		http.Error(w, "missing or invalid repo_ids array", http.StatusBadRequest)
		return
	}

	// Single-tenant guard: only auth.DefaultTeamID exists today. Replace
	// with a repositories.team_id ownership check once teams are persisted.
	claims, ok := r.Context().Value(ClaimsContextKey).(*auth.Claims)
	if !ok || claims.TeamID != auth.DefaultTeamID {
		http.Error(w, "team is not authorized for these repositories", http.StatusForbidden)
		return
	}

	reposMap := make(map[string]*git.Repository)
	for _, id := range strings.Split(repoIDsParam, ",") {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if repo, err := s.openRepository(r.Context(), id); err == nil {
			reposMap[id] = repo
		}
	}
	if len(reposMap) == 0 {
		http.Error(w, "no valid repositories found for the given repo_ids", http.StatusNotFound)
		return
	}

	nodes, err := gitengine.ExtractUnifiedTopology(reposMap)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(nodes)
}

// AddRoutes registers all REST endpoints on mux: mock login, the GitHub
// OAuth2 pair documented in docs/apis_doc.md, and the JWT-protected
// topology extractor.
func (s *APIServer) AddRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/auth/login", s.LoginMock)
	mux.HandleFunc("/api/v1/auth/github/login", auth.HandleLogin)
	mux.HandleFunc("/api/v1/auth/github/callback", auth.HandleCallback)
	mux.HandleFunc("/api/v1/topology", RequireAuth(s.ServeTopology))
}
