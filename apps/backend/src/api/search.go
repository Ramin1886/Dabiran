package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ramin1886/git-interactive-history/backend/src/auth"
	"github.com/ramin1886/git-interactive-history/backend/src/search"
)

// searchResponse is the JSON body of GET /api/v1/search: {"hits":[...]}. It
// mirrors the SearchHit shape in packages/shared-types/src/index.ts.
type searchResponse struct {
	Hits []search.SearchHit `json:"hits"`
}

// ServeSearch handles GET /api/v1/search?q=<text>&repo_ids=1,2 — it runs a
// full-text query over indexed commits scoped to the caller's repositories.
// Requires a valid JWT (enforced by RequireAuth) and applies the same team
// authorization as ServeTopology: every requested repo_id must belong to the
// caller's team. Responds 503 when the search backend is unavailable.
func (s *APIServer) ServeSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		http.Error(w, "missing query parameter q", http.StatusBadRequest)
		return
	}

	claims, ok := r.Context().Value(ClaimsContextKey).(*auth.Claims)
	if !ok {
		http.Error(w, "authorization required", http.StatusUnauthorized)
		return
	}

	requestedIDs := make([]string, 0)
	if repoIDsParam := r.URL.Query().Get("repo_ids"); repoIDsParam != "" {
		for _, id := range strings.Split(repoIDsParam, ",") {
			if id = strings.TrimSpace(id); id != "" {
				requestedIDs = append(requestedIDs, id)
			}
		}
	}

	// Same authorization model as ServeTopology.
	if s.DB != nil {
		if err := s.authorizeRepos(r.Context(), claims.TeamID, requestedIDs); err != nil {
			http.Error(w, "team is not authorized for these repositories", http.StatusForbidden)
			return
		}
	} else if claims.TeamID != auth.DefaultTeamID {
		http.Error(w, "team is not authorized for these repositories", http.StatusForbidden)
		return
	}

	if s.Search == nil {
		http.Error(w, "search backend unavailable", http.StatusServiceUnavailable)
		return
	}

	hits, err := s.Search.Search(r.Context(), q, requestedIDs)
	if err != nil {
		http.Error(w, "search backend unavailable", http.StatusServiceUnavailable)
		return
	}
	if hits == nil {
		hits = make([]search.SearchHit, 0)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(searchResponse{Hits: hits})
}
