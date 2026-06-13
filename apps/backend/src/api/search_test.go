package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ramin1886/git-interactive-history/backend/src/auth"
	"github.com/ramin1886/git-interactive-history/backend/src/gitengine"
	"github.com/ramin1886/git-interactive-history/backend/src/search"
)

// fakeSearcher is an injectable Searcher for handler tests; no live Meili.
type fakeSearcher struct {
	hits []search.SearchHit
	err  error
}

func (f *fakeSearcher) IndexCommits(context.Context, string, []gitengine.CommitNode) error { return nil }
func (f *fakeSearcher) Search(context.Context, string, []string) ([]search.SearchHit, error) {
	return f.hits, f.err
}

// searchServer stands up the mux with an injected Searcher and no DB (so the
// default-team authorization path applies).
func searchServer(t *testing.T, searcher Searcher) *httptest.Server {
	t.Helper()
	apiServer := &APIServer{Engine: gitengine.NewGitEngine(t.TempDir()), Search: searcher}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/search", RequireAuth(apiServer.ServeSearch))
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

func TestSearchHandlerReturnsHits(t *testing.T) {
	fake := &fakeSearcher{hits: []search.SearchHit{{Hash: "1_aaa", Message: "fix bug", RepoID: "1"}}}
	ts := searchServer(t, fake)
	token, _ := auth.GenerateToken(auth.DefaultUserID, auth.DefaultTeamID, "Team Owner")

	resp := authedGet(t, ts.URL+"/api/v1/search?q=bug&repo_ids=1", token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body struct {
		Hits []search.SearchHit `json:"hits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Hits) != 1 || body.Hits[0].Hash != "1_aaa" {
		t.Fatalf("unexpected hits: %+v", body.Hits)
	}
}

func TestSearchHandler503WhenBackendErrors(t *testing.T) {
	fake := &fakeSearcher{err: errors.New("meili down")}
	ts := searchServer(t, fake)
	token, _ := auth.GenerateToken(auth.DefaultUserID, auth.DefaultTeamID, "Team Owner")
	resp := authedGet(t, ts.URL+"/api/v1/search?q=bug&repo_ids=1", token)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", resp.StatusCode)
	}
}

func TestSearchHandler503WhenNoSearcher(t *testing.T) {
	ts := searchServer(t, nil)
	token, _ := auth.GenerateToken(auth.DefaultUserID, auth.DefaultTeamID, "Team Owner")
	resp := authedGet(t, ts.URL+"/api/v1/search?q=bug&repo_ids=1", token)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 with nil searcher, got %d", resp.StatusCode)
	}
}

func TestSearchHandlerRequiresToken(t *testing.T) {
	ts := searchServer(t, &fakeSearcher{})
	resp := authedGet(t, ts.URL+"/api/v1/search?q=bug&repo_ids=1", "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", resp.StatusCode)
	}
}

func TestSearchHandlerRequiresQuery(t *testing.T) {
	ts := searchServer(t, &fakeSearcher{})
	token, _ := auth.GenerateToken(auth.DefaultUserID, auth.DefaultTeamID, "Team Owner")
	resp := authedGet(t, ts.URL+"/api/v1/search?repo_ids=1", token)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 without q, got %d", resp.StatusCode)
	}
}

func TestSearchHandlerRejectsForeignTeam(t *testing.T) {
	// DB nil → only the default team is authorized (matches ServeTopology).
	ts := searchServer(t, &fakeSearcher{})
	token, _ := auth.GenerateToken(auth.DefaultUserID, auth.DefaultTeamID+1, "Team Owner")
	resp := authedGet(t, ts.URL+"/api/v1/search?q=bug&repo_ids=1", token)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for foreign team, got %d", resp.StatusCode)
	}
}
