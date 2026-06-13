package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ramin1886/git-interactive-history/backend/src/auth"
)

// doDelete issues a DELETE with an optional bearer token and returns response.
func doDelete(t *testing.T, url, token string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

// cleanupViews registers removal of every canvas_views row owned by userID so
// reruns stay deterministic regardless of which ids the test created.
func cleanupViews(t *testing.T, pool *pgxpool.Pool, userID int) {
	t.Helper()
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM canvas_views WHERE user_id = $1`, userID)
	})
}

func TestCreateViewPersistsAndReturnsContract(t *testing.T) {
	pool, srv := depLinksServer(t)
	ts := startTestServer(t, srv)
	cleanupViews(t, pool, auth.DefaultUserID)
	token, _ := auth.GenerateToken(auth.DefaultUserID, auth.DefaultTeamID, "Team Owner")

	const state = `{"viewport":{"x":1,"y":2,"zoom":1.5},"filters":{"author":"Alice"}}`
	resp := doJSON(t, http.MethodPost, ts.URL+"/api/v1/views", token, map[string]string{
		"name":  "Release overview",
		"state": state,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", resp.StatusCode, readBody(t, resp))
	}

	// Response must be exactly {id, name, state} — no team_id/user_id/created_at.
	body := readBody(t, resp)
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for k := range raw {
		if k != "id" && k != "name" && k != "state" {
			t.Fatalf("unexpected field %q in response: %s", k, body)
		}
	}
	var out canvasViewResponse
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.ID == 0 || out.Name != "Release overview" || out.State != state {
		t.Fatalf("unexpected response: %+v", out)
	}

	// Verify persistence + ownership directly.
	var name, gotState string
	var userID, teamID int
	if err := pool.QueryRow(context.Background(),
		`SELECT name, state, user_id, team_id FROM canvas_views WHERE id = $1`, out.ID).
		Scan(&name, &gotState, &userID, &teamID); err != nil {
		t.Fatalf("read persisted row: %v", err)
	}
	if name != "Release overview" || gotState != state ||
		userID != auth.DefaultUserID || teamID != auth.DefaultTeamID {
		t.Fatalf("persisted row mismatch: name=%q state=%q user=%d team=%d", name, gotState, userID, teamID)
	}
}

func TestCreateViewRejectsEmptyFields(t *testing.T) {
	pool, srv := depLinksServer(t)
	ts := startTestServer(t, srv)
	cleanupViews(t, pool, auth.DefaultUserID)
	token, _ := auth.GenerateToken(auth.DefaultUserID, auth.DefaultTeamID, "Team Owner")

	cases := []struct {
		name string
		body map[string]string
	}{
		{"empty name", map[string]string{"name": "", "state": "{}"}},
		{"empty state", map[string]string{"name": "v", "state": ""}},
		{"both empty", map[string]string{"name": "", "state": ""}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp := doJSON(t, http.MethodPost, ts.URL+"/api/v1/views", token, c.body)
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d", resp.StatusCode)
			}
		})
	}
}

func TestListViewsReturnsOnlyCallersNewestFirst(t *testing.T) {
	pool, srv := depLinksServer(t)
	ts := startTestServer(t, srv)
	cleanupViews(t, pool, auth.DefaultUserID)
	ctx := context.Background()

	// A second user with a view that must NOT appear in the default user's list.
	var otherUser int
	if err := pool.QueryRow(ctx, `INSERT INTO users (email, name) VALUES ('views-other@example.com', 'Other')
		ON CONFLICT (email) DO UPDATE SET name = EXCLUDED.name RETURNING id`).Scan(&otherUser); err != nil {
		t.Fatalf("seed other user: %v", err)
	}
	cleanupViews(t, pool, otherUser)
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, otherUser) })
	if _, err := pool.Exec(ctx,
		`INSERT INTO canvas_views (user_id, team_id, name, state) VALUES ($1, $2, 'foreign', '{}')`,
		otherUser, auth.DefaultTeamID); err != nil {
		t.Fatalf("seed other view: %v", err)
	}

	token, _ := auth.GenerateToken(auth.DefaultUserID, auth.DefaultTeamID, "Team Owner")
	// Create two views for the caller; the second is newer and must come first.
	first := doJSON(t, http.MethodPost, ts.URL+"/api/v1/views", token,
		map[string]string{"name": "first", "state": "{}"})
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("setup first POST: %d", first.StatusCode)
	}
	second := doJSON(t, http.MethodPost, ts.URL+"/api/v1/views", token,
		map[string]string{"name": "second", "state": "{}"})
	if second.StatusCode != http.StatusCreated {
		t.Fatalf("setup second POST: %d", second.StatusCode)
	}

	resp := authedGet(t, ts.URL+"/api/v1/views", token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var out canvasViewsList
	if err := json.Unmarshal([]byte(readBody(t, resp)), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Views) != 2 {
		t.Fatalf("expected exactly 2 caller views, got %d: %+v", len(out.Views), out.Views)
	}
	if out.Views[0].Name != "second" || out.Views[1].Name != "first" {
		t.Fatalf("expected newest-first [second, first], got [%s, %s]", out.Views[0].Name, out.Views[1].Name)
	}
}

func TestDeleteViewOwnAndNotFound(t *testing.T) {
	pool, srv := depLinksServer(t)
	ts := startTestServer(t, srv)
	cleanupViews(t, pool, auth.DefaultUserID)
	ctx := context.Background()
	token, _ := auth.GenerateToken(auth.DefaultUserID, auth.DefaultTeamID, "Team Owner")

	// Create a view, then delete it (204) and confirm it is gone.
	created := doJSON(t, http.MethodPost, ts.URL+"/api/v1/views", token,
		map[string]string{"name": "to-delete", "state": "{}"})
	if created.StatusCode != http.StatusCreated {
		t.Fatalf("setup POST: %d", created.StatusCode)
	}
	var cv canvasViewResponse
	if err := json.Unmarshal([]byte(readBody(t, created)), &cv); err != nil {
		t.Fatalf("decode created: %v", err)
	}

	del := doDelete(t, ts.URL+"/api/v1/views/"+itoa(cv.ID), token)
	if del.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", del.StatusCode)
	}
	var remaining int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM canvas_views WHERE id = $1`, cv.ID).Scan(&remaining); err != nil {
		t.Fatalf("count after delete: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("expected row removed, %d remain", remaining)
	}

	// Deleting the same id again (now non-existent) yields 404.
	gone := doDelete(t, ts.URL+"/api/v1/views/"+itoa(cv.ID), token)
	if gone.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for non-existent id, got %d", gone.StatusCode)
	}

	// A view owned by another user must not be deletable by the caller (404).
	var otherUser int
	if err := pool.QueryRow(ctx, `INSERT INTO users (email, name) VALUES ('views-del-other@example.com', 'Other')
		ON CONFLICT (email) DO UPDATE SET name = EXCLUDED.name RETURNING id`).Scan(&otherUser); err != nil {
		t.Fatalf("seed other user: %v", err)
	}
	cleanupViews(t, pool, otherUser)
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, otherUser) })
	var otherViewID int
	if err := pool.QueryRow(ctx,
		`INSERT INTO canvas_views (user_id, team_id, name, state) VALUES ($1, $2, 'foreign', '{}') RETURNING id`,
		otherUser, auth.DefaultTeamID).Scan(&otherViewID); err != nil {
		t.Fatalf("seed other view: %v", err)
	}

	forbidden := doDelete(t, ts.URL+"/api/v1/views/"+itoa(otherViewID), token)
	if forbidden.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 deleting another user's view, got %d", forbidden.StatusCode)
	}
	// The other user's row must still exist.
	var stillThere int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM canvas_views WHERE id = $1`, otherViewID).Scan(&stillThere); err != nil {
		t.Fatalf("count other view: %v", err)
	}
	if stillThere != 1 {
		t.Fatalf("other user's view was wrongly deleted")
	}
}

func TestViewsRequireAuth(t *testing.T) {
	_, srv := depLinksServer(t)
	ts := startTestServer(t, srv)

	get := authedGet(t, ts.URL+"/api/v1/views", "")
	if get.StatusCode != http.StatusUnauthorized {
		t.Fatalf("GET without JWT expected 401, got %d", get.StatusCode)
	}
	post := doJSON(t, http.MethodPost, ts.URL+"/api/v1/views", "",
		map[string]string{"name": "x", "state": "{}"})
	if post.StatusCode != http.StatusUnauthorized {
		t.Fatalf("POST without JWT expected 401, got %d", post.StatusCode)
	}
	del := doDelete(t, ts.URL+"/api/v1/views/1", "")
	if del.StatusCode != http.StatusUnauthorized {
		t.Fatalf("DELETE without JWT expected 401, got %d", del.StatusCode)
	}
}
