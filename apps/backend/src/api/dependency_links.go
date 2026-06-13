package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/ramin1886/git-interactive-history/backend/src/auth"
)

// dependencyLinkAnnotationType is the annotations.type discriminator used to
// persist auto-generated cross-repository dependency links. Each link is one
// annotations row (repository_id = from_repo, author_id = caller, payload =
// the DependencyLink JSON below).
const dependencyLinkAnnotationType = "dependency"

// DependencyLink is the shared cross-repository dependency contract the Rust
// worker emits and the frontend renders: from_repo depends on to_repo through
// module/package `via`, classified by kind ("go" or "npm"). All fields are
// snake_case strings; repo ids are the numeric repository ids rendered as
// strings to match the worker's directory-name convention.
type DependencyLink struct {
	FromRepo string `json:"from_repo"`
	ToRepo   string `json:"to_repo"`
	Via      string `json:"via"`
	Kind     string `json:"kind"`
}

// dependencyLinksRequest is the POST /api/v1/dependency-links body.
type dependencyLinksRequest struct {
	Links []DependencyLink `json:"links"`
}

// IngestDependencyLinks handles POST /api/v1/dependency-links (JWT required):
// it persists each DependencyLink as an annotations row keyed by from_repo. The
// caller's team must own every link's from_repo — if ANY from_repo is outside
// the team the whole batch is rejected with 403 (no partial writes); otherwise
// all links are stored and {"stored": <n>} is returned. An empty links array
// stores nothing and returns {"stored": 0}.
func (s *APIServer) IngestDependencyLinks(w http.ResponseWriter, r *http.Request) {
	if s.DB == nil {
		http.Error(w, "dependency links require the database", http.StatusServiceUnavailable)
		return
	}
	claims, ok := r.Context().Value(ClaimsContextKey).(*auth.Claims)
	if !ok {
		http.Error(w, "authorization required", http.StatusUnauthorized)
		return
	}

	var req dependencyLinksRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Authorize: every from_repo must belong to the caller's team. Collect the
	// distinct from_repo ids and run the same ownership gate topology uses.
	fromIDs := make([]string, 0, len(req.Links))
	seen := make(map[string]bool, len(req.Links))
	for _, link := range req.Links {
		if link.FromRepo == "" {
			http.Error(w, "each link requires a from_repo", http.StatusBadRequest)
			return
		}
		if !seen[link.FromRepo] {
			seen[link.FromRepo] = true
			fromIDs = append(fromIDs, link.FromRepo)
		}
	}
	if err := s.authorizeRepos(r.Context(), claims.TeamID, fromIDs); err != nil {
		http.Error(w, "team is not authorized for these repositories", http.StatusForbidden)
		return
	}

	stored := 0
	for _, link := range req.Links {
		fromID, err := strconv.Atoi(link.FromRepo)
		if err != nil {
			http.Error(w, "non-numeric from_repo", http.StatusBadRequest)
			return
		}
		payload, err := json.Marshal(link)
		if err != nil {
			http.Error(w, "failed to encode dependency link", http.StatusInternalServerError)
			return
		}
		if _, err := s.DB.Exec(r.Context(),
			`INSERT INTO annotations (repository_id, type, payload, author_id)
			 VALUES ($1, $2, $3, $4)`,
			fromID, dependencyLinkAnnotationType, string(payload), claims.UserID); err != nil {
			http.Error(w, "failed to store dependency link", http.StatusInternalServerError)
			return
		}
		stored++
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]int{"stored": stored})
}

// ListDependencyLinks handles GET /api/v1/dependency-links?repo_ids=1,2 (JWT
// required): it reconstructs the DependencyLinks stored as 'dependency'
// annotations whose repository_id is in repo_ids. Every requested repo id must
// belong to the caller's team (same gate as topology); repo_ids is mandatory.
func (s *APIServer) ListDependencyLinks(w http.ResponseWriter, r *http.Request) {
	if s.DB == nil {
		http.Error(w, "dependency links require the database", http.StatusServiceUnavailable)
		return
	}
	claims, ok := r.Context().Value(ClaimsContextKey).(*auth.Claims)
	if !ok {
		http.Error(w, "authorization required", http.StatusUnauthorized)
		return
	}

	repoIDsParam := r.URL.Query().Get("repo_ids")
	if repoIDsParam == "" {
		http.Error(w, "missing or invalid repo_ids array", http.StatusBadRequest)
		return
	}
	requestedIDs := make([]string, 0)
	numericIDs := make([]int, 0)
	for _, id := range strings.Split(repoIDsParam, ",") {
		if id = strings.TrimSpace(id); id == "" {
			continue
		}
		n, err := strconv.Atoi(id)
		if err != nil {
			http.Error(w, "non-numeric repo id", http.StatusBadRequest)
			return
		}
		requestedIDs = append(requestedIDs, id)
		numericIDs = append(numericIDs, n)
	}
	if len(requestedIDs) == 0 {
		http.Error(w, "missing or invalid repo_ids array", http.StatusBadRequest)
		return
	}

	if err := s.authorizeRepos(r.Context(), claims.TeamID, requestedIDs); err != nil {
		http.Error(w, "team is not authorized for these repositories", http.StatusForbidden)
		return
	}

	rows, err := s.DB.Query(r.Context(),
		`SELECT payload FROM annotations
		 WHERE type = $1 AND repository_id = ANY($2)
		 ORDER BY id`,
		dependencyLinkAnnotationType, numericIDs)
	if err != nil {
		http.Error(w, "failed to list dependency links", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	links := make([]DependencyLink, 0)
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			http.Error(w, "failed to read dependency links", http.StatusInternalServerError)
			return
		}
		var link DependencyLink
		if err := json.Unmarshal([]byte(payload), &link); err != nil {
			// Skip rows whose payload is not a DependencyLink shape rather than
			// failing the whole listing.
			continue
		}
		links = append(links, link)
	}
	if rows.Err() != nil {
		http.Error(w, "failed to read dependency links", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(dependencyLinksRequest{Links: links})
}
