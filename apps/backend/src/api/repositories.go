package api

import (
	"encoding/json"
	"net/http"

	"github.com/ramin1886/git-interactive-history/backend/src/auth"
	"github.com/ramin1886/git-interactive-history/backend/src/crypto"
)

// repositoryRequest is the JSON body accepted by POST /api/v1/repositories.
// AuthSecret is the plaintext PAT or SSH private key; it is encrypted before
// persistence and never returned to clients.
type repositoryRequest struct {
	Name       string `json:"name"`
	URL        string `json:"url"`
	AuthType   string `json:"auth_type"`
	AuthSecret string `json:"auth_secret"`
}

// repositoryResponse is the credential-free public view of a repository
// returned by the create and list endpoints.
type repositoryResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

// CreateRepository handles POST /api/v1/repositories (Team Owner only): it
// validates auth_type, encrypts auth_secret under the master key, inserts a
// repositories row scoped to the caller's team, and returns the credential-free
// view with 201. The credential is never echoed back.
func (s *APIServer) CreateRepository(w http.ResponseWriter, r *http.Request) {
	if s.DB == nil {
		http.Error(w, "repository registration requires the database", http.StatusServiceUnavailable)
		return
	}
	claims, ok := r.Context().Value(ClaimsContextKey).(*auth.Claims)
	if !ok {
		http.Error(w, "authorization required", http.StatusUnauthorized)
		return
	}

	var req repositoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.URL == "" {
		http.Error(w, "name and url are required", http.StatusBadRequest)
		return
	}
	switch req.AuthType {
	case "", "https", "ssh":
	default:
		http.Error(w, `auth_type must be one of "https", "ssh", or ""`, http.StatusBadRequest)
		return
	}

	var encrypted string
	if req.AuthSecret != "" {
		key, err := crypto.MasterKey()
		if err != nil {
			http.Error(w, "credential encryption unavailable", http.StatusInternalServerError)
			return
		}
		encrypted, err = crypto.Encrypt([]byte(req.AuthSecret), key)
		if err != nil {
			http.Error(w, "failed to encrypt credential", http.StatusInternalServerError)
			return
		}
	}

	var id int
	err := s.DB.QueryRow(r.Context(),
		`INSERT INTO repositories (team_id, name, url, auth_type, encrypted_credential)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		claims.TeamID, req.Name, req.URL, req.AuthType, encrypted).Scan(&id)
	if err != nil {
		http.Error(w, "failed to register repository", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(repositoryResponse{ID: id, Name: req.Name, URL: req.URL})
}

// ListRepositories handles GET /api/v1/repositories (any authenticated user):
// it returns the credential-free repositories scoped to the caller's team.
func (s *APIServer) ListRepositories(w http.ResponseWriter, r *http.Request) {
	if s.DB == nil {
		http.Error(w, "repository listing requires the database", http.StatusServiceUnavailable)
		return
	}
	claims, ok := r.Context().Value(ClaimsContextKey).(*auth.Claims)
	if !ok {
		http.Error(w, "authorization required", http.StatusUnauthorized)
		return
	}

	rows, err := s.DB.Query(r.Context(),
		`SELECT id, name, url FROM repositories WHERE team_id = $1 ORDER BY id`, claims.TeamID)
	if err != nil {
		http.Error(w, "failed to list repositories", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	out := make([]repositoryResponse, 0)
	for rows.Next() {
		var rr repositoryResponse
		if err := rows.Scan(&rr.ID, &rr.Name, &rr.URL); err != nil {
			http.Error(w, "failed to read repositories", http.StatusInternalServerError)
			return
		}
		out = append(out, rr)
	}
	if rows.Err() != nil {
		http.Error(w, "failed to read repositories", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(out)
}
