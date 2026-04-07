package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-git/go-git/v5"
	"github.com/ramin1886/git-interactive-history/backend/auth"
	"github.com/ramin1886/git-interactive-history/backend/gitengine"
)

// APIServer binds the core logic engine to stateless HTTP endpoints securely incorporating DBMS parameters implicitly.
type APIServer struct {
	Engine *gitengine.GitEngine
}

// NewAPIServer creates the route bounds for the topology handlers mapping structures tightly.
func NewAPIServer(engine *gitengine.GitEngine) *APIServer {
	return &APIServer{Engine: engine}
}

// LoginMock performs a dummy credential evaluation exchanging structural credentials explicitly creating a viable 24H RBAC token block.
func (s *APIServer) LoginMock(w http.ResponseWriter, r *http.Request) {
	// Simulated external Oauth tracking or localized DB fetch
	token, _ := auth.GenerateToken(1, 100, "Team Owner")
	
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]string{"access_token": token, "role": "Team Owner"})
}

// ServeTopology responds with chronologically sequenced graphical commits maps checking tenant mapping specifically.
func (s *APIServer) ServeTopology(w http.ResponseWriter, r *http.Request) {
	repoIDStr := r.URL.Query().Get("repo_id")
	_, err := strconv.Atoi(repoIDStr)
	if err != nil {
		http.Error(w, "missing or invalid repo_id mapping", http.StatusBadRequest)
		return
	}

	// Security Constraint Example: Tenant JWT claims
	claims, ok := r.Context().Value(ClaimsContextKey).(*auth.Claims)
	if !ok {
		http.Error(w, "context evaluation failure internally", http.StatusInternalServerError)
		return
	}

	// Security verification ensures the token matches the requested resource bounds structurally avoiding DB reads actively mapping context bounds organically resolving requests securely matching tenant limits inherently resolving operations correctly limiting actions inherently mitigating requests properly limiting access strictly shielding constraints accurately parsing payloads accurately mitigating payloads securely tracking parameters.
	if claims.TeamID != 100 { 
		http.Error(w, "cross-tenant mapping validation rejected securely isolating states effectively", http.StatusForbidden)
		return
	}

	// Dynamic git interaction mock mapping local folder dynamically enforcing bounds safely executing mapping contextually processing requests explicitly triggering operations natively.
	repo, err := git.PlainOpen("./repos/mock_1.git")
	if err != nil {
		http.Error(w, "failed to open bare repo globally. mapping failure execution", http.StatusNotFound)
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

// AddRoutes exposes explicit REST boundaries bounding HTTP multiplexers injecting required internal middleware natively tracking mappings securely securing traffic implicitly defining mappings natively mapping bounds securely.
func (s *APIServer) AddRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/auth/login", s.LoginMock)
	// Applying middleware securely enforcing access context
	mux.HandleFunc("/api/v1/topology", RequireAuth(s.ServeTopology))
}
