package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ramin1886/git-interactive-history/backend/src/auth"
	"github.com/ramin1886/git-interactive-history/backend/src/crypto"
	"github.com/ramin1886/git-interactive-history/backend/src/db"
	"github.com/ramin1886/git-interactive-history/backend/src/gitengine"
)

// readBody drains resp.Body and returns it as a string.
func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(b)
}

// itoa is a local shorthand for strconv.Itoa to keep query construction terse.
func itoa(n int) string { return strconv.Itoa(n) }

// dbServer stands up the full mux backed by the live database pool, seeding a
// throwaway user+team and returning the server plus that team id. All seeded
// rows are removed on cleanup so reruns stay deterministic.
func dbServer(t *testing.T) (*httptest.Server, int) {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set; skipping live database test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(pool.Close)

	email := "repo-api-test-owner@example.com"
	var userID, teamID int
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email, name) VALUES ($1, 'Repo Test')
		 ON CONFLICT (email) DO UPDATE SET name = EXCLUDED.name RETURNING id`,
		email).Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO teams (name, owner_id) VALUES ('repo-api-test-team', $1)
		 ON CONFLICT (name) DO UPDATE SET owner_id = EXCLUDED.owner_id RETURNING id`,
		userID).Scan(&teamID); err != nil {
		t.Fatalf("seed team: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM repositories WHERE team_id = $1`, teamID)
		pool.Exec(ctx, `DELETE FROM teams WHERE id = $1`, teamID)
		pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
	})

	apiServer := NewAPIServer(gitengine.NewGitEngine(t.TempDir()), pool)
	mux := http.NewServeMux()
	apiServer.AddRoutes(mux)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts, teamID
}

// doJSON issues a request with an optional bearer token and a JSON body.
func doJSON(t *testing.T, method, url, token string, body interface{}) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req, err := http.NewRequest(method, url, &buf)
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

func TestCreateRepositoryHappyPath(t *testing.T) {
	t.Setenv("REPO_CRED_KEY", "") // use deterministic dev master key
	ts, teamID := dbServer(t)
	token, _ := auth.GenerateToken(1, teamID, "Team Owner")

	resp := doJSON(t, http.MethodPost, ts.URL+"/api/v1/repositories", token, map[string]string{
		"name":        "core",
		"url":         "https://github.com/example/core.git",
		"auth_type":   "https",
		"auth_secret": "ghp_secret_pat_value",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	bodyBytes := readBody(t, resp)
	if strings.Contains(bodyBytes, "ghp_secret_pat_value") || strings.Contains(bodyBytes, "auth_secret") || strings.Contains(bodyBytes, "encrypted") {
		t.Fatalf("credential leaked into create response: %s", bodyBytes)
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(bodyBytes), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	id, ok := out["id"].(float64)
	if !ok || id == 0 {
		t.Fatalf("missing id in response: %v", out)
	}

	// The stored credential must decrypt back to the plaintext.
	pool, _ := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	defer pool.Close()
	var stored, authType string
	if err := pool.QueryRow(context.Background(),
		"SELECT encrypted_credential, auth_type FROM repositories WHERE id = $1", int(id)).Scan(&stored, &authType); err != nil {
		t.Fatalf("read stored cred: %v", err)
	}
	if authType != "https" {
		t.Fatalf("auth_type not persisted: %q", authType)
	}
	if stored == "ghp_secret_pat_value" {
		t.Fatal("credential stored in plaintext")
	}
	key, _ := crypto.MasterKey()
	dec, err := crypto.Decrypt(stored, key)
	if err != nil || string(dec) != "ghp_secret_pat_value" {
		t.Fatalf("stored credential does not decrypt: dec=%q err=%v", dec, err)
	}
}

func TestCreateRepositoryRejectsNonOwner(t *testing.T) {
	ts, teamID := dbServer(t)
	token, _ := auth.GenerateToken(1, teamID, "Team Member")
	resp := doJSON(t, http.MethodPost, ts.URL+"/api/v1/repositories", token, map[string]string{
		"name": "core", "url": "https://github.com/example/core.git",
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for non-owner, got %d", resp.StatusCode)
	}
}

func TestCreateRepositoryRejectsBadAuthType(t *testing.T) {
	ts, teamID := dbServer(t)
	token, _ := auth.GenerateToken(1, teamID, "Team Owner")
	resp := doJSON(t, http.MethodPost, ts.URL+"/api/v1/repositories", token, map[string]string{
		"name": "core", "url": "https://x.git", "auth_type": "smtp",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad auth_type, got %d", resp.StatusCode)
	}
}

func TestListRepositoriesIsTeamScoped(t *testing.T) {
	t.Setenv("REPO_CRED_KEY", "")
	ts, teamID := dbServer(t)
	ownerToken, _ := auth.GenerateToken(1, teamID, "Team Owner")
	doJSON(t, http.MethodPost, ts.URL+"/api/v1/repositories", ownerToken, map[string]string{
		"name": "alpha", "url": "https://github.com/example/alpha.git",
	})

	// A member of the same team can list and sees the repo, without credentials.
	memberToken, _ := auth.GenerateToken(2, teamID, "Team Member")
	resp := doJSON(t, http.MethodGet, ts.URL+"/api/v1/repositories", memberToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body := readBody(t, resp)
	if !strings.Contains(body, "alpha") {
		t.Fatalf("own-team repo missing from list: %s", body)
	}
	if strings.Contains(body, "credential") || strings.Contains(body, "auth_secret") {
		t.Fatalf("credential field leaked into list: %s", body)
	}

	// A caller on a different team sees an empty list.
	otherToken, _ := auth.GenerateToken(3, teamID+99999, "Team Owner")
	resp2 := doJSON(t, http.MethodGet, ts.URL+"/api/v1/repositories", otherToken, nil)
	body2 := readBody(t, resp2)
	if strings.Contains(body2, "alpha") {
		t.Fatalf("foreign team saw repo: %s", body2)
	}
}

func TestTopologyAuthorizationByTeamOwnership(t *testing.T) {
	t.Setenv("REPO_CRED_KEY", "")
	ts, teamID := dbServer(t)
	ownerToken, _ := auth.GenerateToken(1, teamID, "Team Owner")
	resp := doJSON(t, http.MethodPost, ts.URL+"/api/v1/repositories", ownerToken, map[string]string{
		"name": "topo", "url": "https://github.com/example/topo.git",
	})
	var created map[string]interface{}
	json.Unmarshal([]byte(readBody(t, resp)), &created)
	repoID := int(created["id"].(float64))

	// Own-team request: passes the ownership check (the repo fails to clone, so
	// the response is 404 "no valid repositories", NOT 403 — authorization
	// succeeded). A 403 here would prove the ownership gate wrongly rejected.
	own := authedGet(t, ts.URL+"/api/v1/topology?repo_ids="+itoa(repoID), ownerToken)
	if own.StatusCode == http.StatusForbidden {
		t.Fatalf("own-team repo wrongly rejected with 403")
	}

	// Foreign-team request for the same repo id: must be 403.
	foreignToken, _ := auth.GenerateToken(9, teamID+99999, "Team Owner")
	foreign := authedGet(t, ts.URL+"/api/v1/topology?repo_ids="+itoa(repoID), foreignToken)
	if foreign.StatusCode != http.StatusForbidden {
		t.Fatalf("foreign-team repo expected 403, got %d", foreign.StatusCode)
	}
}
