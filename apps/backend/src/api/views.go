package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/ramin1886/git-interactive-history/backend/src/auth"
)

// canvasViewRequest is the JSON body accepted by POST /api/v1/views. Both
// fields are required; state is an opaque, frontend-owned JSON string the
// backend persists verbatim (only its non-emptiness is validated).
type canvasViewRequest struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

// canvasViewResponse is the public view of a saved canvas view returned by the
// create and list endpoints. team_id, user_id, and created_at are intentionally
// omitted — the contract is exactly {id, name, state} (packages/shared-types
// CanvasView).
type canvasViewResponse struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
}

// canvasViewsList is the GET /api/v1/views envelope: {"views":[...]}.
type canvasViewsList struct {
	Views []canvasViewResponse `json:"views"`
}

// HandleViews dispatches the /api/v1/views collection by method: POST creates a
// view for the caller, GET lists the caller's views. All other methods get 405.
// Both sub-handlers require a valid JWT (registered behind RequireAuth) and the
// database pool — the canvas-view endpoints are persistence-only and return 503
// when s.DB is nil.
func (s *APIServer) HandleViews(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.CreateView(w, r)
	case http.MethodGet:
		s.ListViews(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// CreateView handles POST /api/v1/views (JWT required): it validates that name
// and state are both non-empty, inserts a canvas_views row owned by the caller
// (user_id=claims.UserID, team_id=claims.TeamID), and returns 201 with the
// {id, name, state} view. state is stored verbatim as an opaque JSON string.
// Requires the database pool; returns 503 when s.DB is nil.
func (s *APIServer) CreateView(w http.ResponseWriter, r *http.Request) {
	if s.DB == nil {
		http.Error(w, "saved canvas views require the database", http.StatusServiceUnavailable)
		return
	}
	claims, ok := r.Context().Value(ClaimsContextKey).(*auth.Claims)
	if !ok {
		http.Error(w, "authorization required", http.StatusUnauthorized)
		return
	}

	var req canvasViewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.State == "" {
		http.Error(w, "name and state are required", http.StatusBadRequest)
		return
	}

	var id int
	if err := s.DB.QueryRow(r.Context(),
		`INSERT INTO canvas_views (user_id, team_id, name, state)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		claims.UserID, claims.TeamID, req.Name, req.State).Scan(&id); err != nil {
		http.Error(w, "failed to save canvas view", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(canvasViewResponse{ID: id, Name: req.Name, State: req.State})
}

// ListViews handles GET /api/v1/views (JWT required): it returns
// {"views":[...]} containing only the caller's own saved views (user_id =
// claims.UserID), newest first. Requires the database pool; returns 503 when
// s.DB is nil.
func (s *APIServer) ListViews(w http.ResponseWriter, r *http.Request) {
	if s.DB == nil {
		http.Error(w, "saved canvas views require the database", http.StatusServiceUnavailable)
		return
	}
	claims, ok := r.Context().Value(ClaimsContextKey).(*auth.Claims)
	if !ok {
		http.Error(w, "authorization required", http.StatusUnauthorized)
		return
	}

	rows, err := s.DB.Query(r.Context(),
		`SELECT id, name, state FROM canvas_views WHERE user_id = $1 ORDER BY id DESC`,
		claims.UserID)
	if err != nil {
		http.Error(w, "failed to list canvas views", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	views := make([]canvasViewResponse, 0)
	for rows.Next() {
		var v canvasViewResponse
		if err := rows.Scan(&v.ID, &v.Name, &v.State); err != nil {
			http.Error(w, "failed to read canvas views", http.StatusInternalServerError)
			return
		}
		views = append(views, v)
	}
	if rows.Err() != nil {
		http.Error(w, "failed to read canvas views", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(canvasViewsList{Views: views})
}

// DeleteView handles DELETE /api/v1/views/{id} (JWT required): it deletes the
// row only when it both exists and belongs to claims.UserID, returning 204 on
// success and 404 when the id is unknown or owned by another user (the
// ownership predicate folds "not found" and "not yours" into one response so it
// leaks nothing about other users' views). The trailing numeric id is parsed
// from the path with the same /prefix/<segment> split the ws relay uses for
// room ids (roomFromRequest). Requires the database pool; returns 503 when
// s.DB is nil.
func (s *APIServer) DeleteView(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.DB == nil {
		http.Error(w, "saved canvas views require the database", http.StatusServiceUnavailable)
		return
	}
	claims, ok := r.Context().Value(ClaimsContextKey).(*auth.Claims)
	if !ok {
		http.Error(w, "authorization required", http.StatusUnauthorized)
		return
	}

	id, err := viewIDFromPath(r.URL.Path)
	if err != nil {
		http.Error(w, "invalid view id", http.StatusBadRequest)
		return
	}

	tag, err := s.DB.Exec(r.Context(),
		`DELETE FROM canvas_views WHERE id = $1 AND user_id = $2`, id, claims.UserID)
	if err != nil {
		http.Error(w, "failed to delete canvas view", http.StatusInternalServerError)
		return
	}
	if tag.RowsAffected() == 0 {
		http.Error(w, "canvas view not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusNoContent)
}

// viewIDFromPath extracts the trailing numeric id from a /api/v1/views/<id>
// path, tolerating a trailing slash. It mirrors the ws relay's path-segment
// split (src/ws/hub.go roomFromRequest) rather than relying on Go 1.22 method
// patterns, matching how this repo's net/http mux registers routes.
func viewIDFromPath(path string) (int, error) {
	const prefix = "/api/v1/views/"
	idx := strings.Index(path, prefix)
	if idx == -1 {
		return 0, strconv.ErrSyntax
	}
	seg := strings.Trim(path[idx+len(prefix):], "/")
	return strconv.Atoi(seg)
}
