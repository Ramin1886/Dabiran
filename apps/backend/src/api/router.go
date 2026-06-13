// Package api exposes the REST surface documented in docs/apis_doc.md:
// authentication endpoints and the unified topology extractor.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ramin1886/git-interactive-history/backend/src/auth"
	"github.com/ramin1886/git-interactive-history/backend/src/crypto"
	"github.com/ramin1886/git-interactive-history/backend/src/gitengine"
	"github.com/ramin1886/git-interactive-history/backend/src/search"
	"github.com/ramin1886/git-interactive-history/backend/src/secrets"
)

// Searcher is the subset of the Meilisearch client the API depends on. It is
// an interface so handlers can be tested against a fake without a live Meili
// (search.Client satisfies it).
type Searcher interface {
	IndexCommits(ctx context.Context, repoID string, nodes []gitengine.CommitNode) error
	Search(ctx context.Context, q string, repoIDs []string) ([]search.SearchHit, error)
}

// APIServer binds the git engine and (optionally) the database pool to the
// stateless HTTP handlers.
type APIServer struct {
	Engine *gitengine.GitEngine
	// DB is optional: when nil (e.g. local dev without Postgres) repository
	// lookups fall back to bare repos under Engine.BaseStoragePath.
	DB *pgxpool.Pool
	// OAuth serves the GitHub OAuth2 endpoints; it shares the same DB pool.
	OAuth *auth.OAuthHandler
	// Search indexes and queries commits via Meilisearch. It may be nil to
	// disable full-text search (the /api/v1/search endpoint then returns 503
	// and topology indexing is skipped).
	Search Searcher
	// RepoSyncer optionally overrides the git engine used by the webhook
	// handler to fetch new commits. When nil the handler uses Engine; tests
	// inject a stub so the handler is exercisable without network access.
	RepoSyncer repoSyncer
}

// NewAPIServer constructs an APIServer over engine. pool may be nil to run
// without database-backed repository metadata; the OAuth handler is wired to
// the same pool so identity persistence and repository scoping agree. The
// Meilisearch client is built from the environment (MEILI_URL/MEILI_MASTER_KEY)
// and degrades gracefully when Meili is down.
func NewAPIServer(engine *gitengine.GitEngine, pool *pgxpool.Pool) *APIServer {
	return &APIServer{
		Engine: engine,
		DB:     pool,
		OAuth:  auth.NewOAuthHandler(pool),
		Search: search.NewClient(),
	}
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
// the engine, decrypting the stored credential (when present) under the master
// key and passing auth_type + secret to the engine; an empty credential
// triggers an anonymous fetch. Otherwise it falls back to a local bare repo at
// <BaseStoragePath>/mock_<id>.git, then <BaseStoragePath>/repo_<id>.git.
func (s *APIServer) openRepository(ctx context.Context, id string) (*git.Repository, error) {
	if s.DB != nil {
		if numericID, err := strconv.Atoi(id); err == nil {
			var url, authType, encrypted string
			row := s.DB.QueryRow(ctx, "SELECT url, auth_type, encrypted_credential FROM repositories WHERE id = $1", numericID)
			if err := row.Scan(&url, &authType, &encrypted); err == nil {
				secret := ""
				if encrypted != "" {
					key, kerr := secrets.ResolveMasterKey(ctx)
					if kerr != nil {
						return nil, kerr
					}
					plain, derr := crypto.Decrypt(encrypted, key)
					if derr != nil {
						return nil, derr
					}
					secret = string(plain)
				}
				return s.Engine.EnsureRepository(ctx, numericID, url, authType, secret)
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

// authorizeRepos returns nil only when every id in requestedIDs is a numeric
// repository owned by teamID. Non-numeric ids and any id not owned by the team
// cause an error (mapped to 403 by the caller). An empty request is allowed
// (the caller separately rejects empty repo_ids upstream).
func (s *APIServer) authorizeRepos(ctx context.Context, teamID int, requestedIDs []string) error {
	numeric := make([]int, 0, len(requestedIDs))
	for _, id := range requestedIDs {
		n, err := strconv.Atoi(id)
		if err != nil {
			return fmt.Errorf("non-numeric repo id %q", id)
		}
		numeric = append(numeric, n)
	}
	if len(numeric) == 0 {
		return nil
	}
	rows, err := s.DB.Query(ctx, "SELECT id FROM repositories WHERE team_id = $1 AND id = ANY($2)", teamID, numeric)
	if err != nil {
		return err
	}
	defer rows.Close()
	owned := make(map[int]bool, len(numeric))
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return err
		}
		owned[id] = true
	}
	if rows.Err() != nil {
		return rows.Err()
	}
	for _, n := range numeric {
		if !owned[n] {
			return fmt.Errorf("repository %d not owned by team %d", n, teamID)
		}
	}
	return nil
}

// ServeTopology handles GET /api/v1/topology?repo_ids=1,2 — it resolves each
// repository, extracts the unified chronological topology, and writes the
// CommitNode JSON array. Requires a valid JWT (enforced by RequireAuth).
//
// Authorization has two modes:
//   - DB-backed: every requested repo_id must belong to the caller's team
//     (SELECT ... WHERE team_id=$1 AND id = ANY($2)); any requested id the team
//     does not own yields 403.
//   - DB nil (local dev with filesystem-seeded repos): the legacy single-tenant
//     guard applies — only auth.DefaultTeamID is authorized.
func (s *APIServer) ServeTopology(w http.ResponseWriter, r *http.Request) {
	repoIDsParam := r.URL.Query().Get("repo_ids")
	if repoIDsParam == "" {
		http.Error(w, "missing or invalid repo_ids array", http.StatusBadRequest)
		return
	}

	claims, ok := r.Context().Value(ClaimsContextKey).(*auth.Claims)
	if !ok {
		http.Error(w, "authorization required", http.StatusUnauthorized)
		return
	}

	requestedIDs := make([]string, 0)
	for _, id := range strings.Split(repoIDsParam, ",") {
		if id = strings.TrimSpace(id); id != "" {
			requestedIDs = append(requestedIDs, id)
		}
	}

	if s.DB != nil {
		if err := s.authorizeRepos(r.Context(), claims.TeamID, requestedIDs); err != nil {
			http.Error(w, "team is not authorized for these repositories", http.StatusForbidden)
			return
		}
	} else if claims.TeamID != auth.DefaultTeamID {
		// Filesystem-seeded dev mode: only the default team is authorized.
		http.Error(w, "team is not authorized for these repositories", http.StatusForbidden)
		return
	}

	reposMap := make(map[string]*git.Repository)
	for _, id := range requestedIDs {
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

	// Best-effort full-text indexing of the freshly extracted commits, one
	// goroutine per repo. Index the un-aggregated nodes so search always
	// covers real commits. Fire-and-forget: never blocks the response.
	if s.Search != nil {
		byRepo := make(map[string][]gitengine.CommitNode)
		for _, n := range nodes {
			byRepo[n.RepoID] = append(byRepo[n.RepoID], n)
		}
		for repoID, repoNodes := range byRepo {
			go func(id string, ns []gitengine.CommitNode) {
				if err := s.Search.IndexCommits(context.Background(), id, ns); err != nil {
					log.Printf("topology: background index for repo %s failed: %v", id, err)
				}
			}(repoID, repoNodes)
		}
	}

	// Optional aggregation: collapse maximal linear runs when max_nodes is set
	// and the extracted count exceeds it. Absent/zero/over-budget leaves every
	// node kind="commit", count=1 (no behaviour change).
	if maxParam := r.URL.Query().Get("max_nodes"); maxParam != "" {
		if maxNodes, perr := strconv.Atoi(maxParam); perr == nil && maxNodes > 0 {
			nodes = gitengine.AggregateLinearRuns(nodes, maxNodes)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(nodes)
}

// AddRoutes registers all REST endpoints on mux: mock login, the GitHub
// OAuth2 pair documented in docs/apis_doc.md, repository management (create
// is Team Owner only; list is any authenticated user), the JWT-protected
// topology extractor and full-text search, and the (HMAC-authenticated, no
// JWT) GitHub webhook ingress.
func (s *APIServer) AddRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/auth/login", s.LoginMock)
	mux.HandleFunc("/api/v1/auth/github/login", s.OAuth.HandleLogin)
	mux.HandleFunc("/api/v1/auth/github/callback", s.OAuth.HandleCallback)
	mux.HandleFunc("/api/v1/repositories", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			RequireRole("Team Owner", s.CreateRepository)(w, r)
		case http.MethodGet:
			RequireAuth(s.ListRepositories)(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/v1/topology", RequireAuth(s.ServeTopology))
	mux.HandleFunc("/api/v1/dependency-links", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			RequireAuth(s.IngestDependencyLinks)(w, r)
		case http.MethodGet:
			RequireAuth(s.ListDependencyLinks)(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	// Saved canvas views (per-user). The collection (POST create / GET list)
	// lives at /api/v1/views; the trailing-slash pattern owns DELETE /{id}.
	// Both require a valid JWT.
	mux.HandleFunc("/api/v1/views", RequireAuth(s.HandleViews))
	mux.HandleFunc("/api/v1/views/", RequireAuth(s.DeleteView))
	mux.HandleFunc("/api/v1/rooms/", RequireAuth(s.CompactRoom))
	mux.HandleFunc("/api/v1/search", RequireAuth(s.ServeSearch))
	// GitHub signs webhook bodies (HMAC), so this endpoint is intentionally
	// not behind RequireAuth.
	mux.HandleFunc("/api/v1/webhooks/github", s.HandleGitHubWebhook)
}
