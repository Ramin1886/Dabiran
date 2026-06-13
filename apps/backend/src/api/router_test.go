package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/ramin1886/git-interactive-history/backend/src/auth"
	"github.com/ramin1886/git-interactive-history/backend/src/gitengine"
)

// buildBareFixture creates a bare repository with two commits at dest.
func buildBareFixture(t *testing.T, dest string) {
	t.Helper()
	srcDir := t.TempDir()
	repo, err := git.PlainInit(srcDir, false)
	if err != nil {
		t.Fatalf("PlainInit: %v", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}
	base := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	for i, msg := range []string{"first", "second"} {
		if err := os.WriteFile(filepath.Join(srcDir, "f.txt"), []byte(msg), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
		if _, err := wt.Add("f.txt"); err != nil {
			t.Fatalf("add: %v", err)
		}
		sig := &object.Signature{Name: "Bob", Email: "bob@example.com", When: base.Add(time.Duration(i) * time.Minute)}
		if _, err := wt.Commit(msg, &git.CommitOptions{Author: sig, Committer: sig}); err != nil {
			t.Fatalf("commit: %v", err)
		}
	}
	bare, err := git.PlainClone(dest, true, &git.CloneOptions{URL: srcDir})
	if err != nil {
		t.Fatalf("bare clone: %v", err)
	}
	err = bare.Fetch(&git.FetchOptions{RefSpecs: []config.RefSpec{"+refs/heads/*:refs/heads/*"}, Force: true})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		t.Fatalf("fetch: %v", err)
	}
}

// newTestServer stands up the full mux (routes + middleware) over a temp
// storage dir containing mock_1.git, without a database pool.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	storage := t.TempDir()
	buildBareFixture(t, filepath.Join(storage, "mock_1.git"))
	apiServer := NewAPIServer(gitengine.NewGitEngine(storage), nil)
	mux := http.NewServeMux()
	apiServer.AddRoutes(mux)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

// authedGet issues a GET with an optional bearer token and returns response.
func authedGet(t *testing.T, url, token string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
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

func TestServeTopologyEndToEnd(t *testing.T) {
	ts := newTestServer(t)
	token, err := auth.GenerateToken(auth.DefaultUserID, auth.DefaultTeamID, "Team Owner")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	resp := authedGet(t, ts.URL+"/api/v1/topology?repo_ids=1", token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var nodes []gitengine.CommitNode
	if err := json.NewDecoder(resp.Body).Decode(&nodes); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}
	for _, n := range nodes {
		if !strings.HasPrefix(n.Hash, "1_") || n.RepoID != "1" {
			t.Fatalf("node not prefixed for repo 1: %+v", n)
		}
	}
	if nodes[0].XOffset != 0 || nodes[1].XOffset <= 0 {
		t.Fatalf("x_offset layout missing: %v, %v", nodes[0].XOffset, nodes[1].XOffset)
	}
}

func TestServeTopologyRequiresToken(t *testing.T) {
	ts := newTestServer(t)
	resp := authedGet(t, ts.URL+"/api/v1/topology?repo_ids=1", "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", resp.StatusCode)
	}
}

func TestServeTopologyRequiresRepoIDs(t *testing.T) {
	ts := newTestServer(t)
	token, _ := auth.GenerateToken(auth.DefaultUserID, auth.DefaultTeamID, "Team Owner")
	resp := authedGet(t, ts.URL+"/api/v1/topology", token)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 without repo_ids, got %d", resp.StatusCode)
	}
}

func TestServeTopologyUnknownRepoIs404(t *testing.T) {
	ts := newTestServer(t)
	token, _ := auth.GenerateToken(auth.DefaultUserID, auth.DefaultTeamID, "Team Owner")
	resp := authedGet(t, ts.URL+"/api/v1/topology?repo_ids=999", token)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown repo, got %d", resp.StatusCode)
	}
}

func TestServeTopologyRejectsForeignTeam(t *testing.T) {
	ts := newTestServer(t)
	token, _ := auth.GenerateToken(auth.DefaultUserID, auth.DefaultTeamID+1, "Team Owner")
	resp := authedGet(t, ts.URL+"/api/v1/topology?repo_ids=1", token)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for foreign team, got %d", resp.StatusCode)
	}
}

func TestLoginMockIssuesValidToken(t *testing.T) {
	ts := newTestServer(t)
	resp, err := http.Get(ts.URL + "/api/v1/auth/login")
	if err != nil {
		t.Fatalf("login request: %v", err)
	}
	defer resp.Body.Close()
	var payload map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload["role"] == "" {
		t.Fatal("login response missing role")
	}
	claims, err := auth.ValidateToken(payload["access_token"])
	if err != nil {
		t.Fatalf("mock token invalid: %v", err)
	}
	if claims.TeamID != auth.DefaultTeamID {
		t.Fatalf("mock token team: got %d want %d", claims.TeamID, auth.DefaultTeamID)
	}
}

func TestOAuthRoutesRegistered(t *testing.T) {
	ts := newTestServer(t)
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Get(ts.URL + "/api/v1/auth/github/login")
	if err != nil {
		t.Fatalf("oauth login request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTemporaryRedirect {
		t.Fatalf("expected 307 from oauth login, got %d", resp.StatusCode)
	}
}
