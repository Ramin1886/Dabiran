package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ramin1886/git-interactive-history/backend/src/auth"
	"github.com/ramin1886/git-interactive-history/backend/src/db"
	"github.com/ramin1886/git-interactive-history/backend/src/gitengine"
)

// startTestServer mounts srv's routes on an httptest.Server with cleanup.
func startTestServer(t *testing.T, srv *APIServer) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	srv.AddRoutes(mux)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

// depLinksServer stands up the full mux backed by the live database, seeded
// with the single-tenant default identity (auth.DefaultUserID / team
// auth.DefaultTeamID) so dependency-link writes satisfy the users and teams
// foreign keys. It returns the server and a cleanup-registered pool. Any rows
// the test creates (repositories, annotations) are the caller's responsibility
// to remove; the helper only owns the seed identity, which is shared/idempotent
// and therefore intentionally left in place.
func depLinksServer(t *testing.T) (*pgxpool.Pool, *APIServer) {
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
	if err := db.SeedSingleTenant(ctx, pool, auth.DefaultUserID, auth.DefaultTeamID); err != nil {
		t.Fatalf("seed single tenant: %v", err)
	}
	t.Cleanup(pool.Close)

	srv := NewAPIServer(gitengine.NewGitEngine(t.TempDir()), pool)
	return pool, srv
}

// seedRepo inserts a repositories row for teamID and registers cleanup of it
// and its annotations. It returns the new numeric id.
func seedRepo(t *testing.T, pool *pgxpool.Pool, teamID int, name string) int {
	t.Helper()
	ctx := context.Background()
	var id int
	if err := pool.QueryRow(ctx,
		`INSERT INTO repositories (team_id, name, url) VALUES ($1, $2, 'https://example.test/x.git') RETURNING id`,
		teamID, name).Scan(&id); err != nil {
		t.Fatalf("seed repo: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM annotations WHERE repository_id = $1`, id)
		pool.Exec(ctx, `DELETE FROM repositories WHERE id = $1`, id)
	})
	return id
}

func TestIngestDependencyLinksStoresRows(t *testing.T) {
	pool, srv := depLinksServer(t)
	ts := startTestServer(t, srv)

	fromID := seedRepo(t, pool, auth.DefaultTeamID, "dep-from")
	toID := seedRepo(t, pool, auth.DefaultTeamID, "dep-to")
	token, _ := auth.GenerateToken(auth.DefaultUserID, auth.DefaultTeamID, "Team Owner")

	resp := doJSON(t, http.MethodPost, ts.URL+"/api/v1/dependency-links", token, map[string]any{
		"links": []map[string]string{
			{"from_repo": itoa(fromID), "to_repo": itoa(toID), "via": "github.com/acme/lib", "kind": "go"},
		},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, readBody(t, resp))
	}
	var out map[string]int
	if err := json.Unmarshal([]byte(readBody(t, resp)), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out["stored"] != 1 {
		t.Fatalf("expected stored=1, got %d", out["stored"])
	}

	// Verify the persisted annotations row shape directly.
	var typ, payload string
	var repoID, authorID int
	if err := pool.QueryRow(context.Background(),
		`SELECT repository_id, type, payload, author_id FROM annotations WHERE repository_id = $1 AND type = 'dependency'`,
		fromID).Scan(&repoID, &typ, &payload, &authorID); err != nil {
		t.Fatalf("read annotation: %v", err)
	}
	if repoID != fromID || typ != "dependency" || authorID != auth.DefaultUserID {
		t.Fatalf("unexpected row: repo=%d type=%q author=%d", repoID, typ, authorID)
	}
	var link DependencyLink
	if err := json.Unmarshal([]byte(payload), &link); err != nil {
		t.Fatalf("payload not a DependencyLink: %v (%s)", err, payload)
	}
	if link.Via != "github.com/acme/lib" || link.Kind != "go" || link.ToRepo != itoa(toID) {
		t.Fatalf("payload mismatch: %+v", link)
	}
}

func TestIngestDependencyLinksForbiddenWhenFromRepoOutsideTeam(t *testing.T) {
	pool, srv := depLinksServer(t)
	ts := startTestServer(t, srv)

	// A repo owned by a DIFFERENT team; the default-team caller must not be
	// able to attach dependency links keyed to it.
	ctx := context.Background()
	var foreignUser, foreignTeam int
	pool.QueryRow(ctx, `INSERT INTO users (email, name) VALUES ('dep-foreign@example.com', 'F')
		ON CONFLICT (email) DO UPDATE SET name = EXCLUDED.name RETURNING id`).Scan(&foreignUser)
	pool.QueryRow(ctx, `INSERT INTO teams (name, owner_id) VALUES ('dep-foreign-team', $1) RETURNING id`, foreignUser).Scan(&foreignTeam)
	foreignRepo := seedRepo(t, pool, foreignTeam, "dep-foreign-repo")
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM teams WHERE id = $1`, foreignTeam)
		pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, foreignUser)
	})

	token, _ := auth.GenerateToken(auth.DefaultUserID, auth.DefaultTeamID, "Team Owner")
	resp := doJSON(t, http.MethodPost, ts.URL+"/api/v1/dependency-links", token, map[string]any{
		"links": []map[string]string{
			{"from_repo": itoa(foreignRepo), "to_repo": "1", "via": "github.com/acme/lib", "kind": "go"},
		},
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for foreign from_repo, got %d", resp.StatusCode)
	}
}

func TestListDependencyLinksReturnsStored(t *testing.T) {
	pool, srv := depLinksServer(t)
	ts := startTestServer(t, srv)

	fromID := seedRepo(t, pool, auth.DefaultTeamID, "dep-list-from")
	toID := seedRepo(t, pool, auth.DefaultTeamID, "dep-list-to")
	token, _ := auth.GenerateToken(auth.DefaultUserID, auth.DefaultTeamID, "Team Owner")

	post := doJSON(t, http.MethodPost, ts.URL+"/api/v1/dependency-links", token, map[string]any{
		"links": []map[string]string{
			{"from_repo": itoa(fromID), "to_repo": itoa(toID), "via": "github.com/acme/lib", "kind": "go"},
		},
	})
	if post.StatusCode != http.StatusOK {
		t.Fatalf("setup POST failed: %d", post.StatusCode)
	}

	resp := authedGet(t, ts.URL+"/api/v1/dependency-links?repo_ids="+itoa(fromID)+","+itoa(toID), token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var out dependencyLinksRequest
	if err := json.Unmarshal([]byte(readBody(t, resp)), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	found := false
	for _, l := range out.Links {
		if l.FromRepo == itoa(fromID) && l.ToRepo == itoa(toID) && l.Via == "github.com/acme/lib" && l.Kind == "go" {
			found = true
		}
	}
	if !found {
		t.Fatalf("stored link not returned: %+v", out.Links)
	}
}

func TestDependencyLinksRequireAuth(t *testing.T) {
	_, srv := depLinksServer(t)
	ts := startTestServer(t, srv)

	get := authedGet(t, ts.URL+"/api/v1/dependency-links?repo_ids=1", "")
	if get.StatusCode != http.StatusUnauthorized {
		t.Fatalf("GET without JWT expected 401, got %d", get.StatusCode)
	}
	post := doJSON(t, http.MethodPost, ts.URL+"/api/v1/dependency-links", "", map[string]any{"links": []any{}})
	if post.StatusCode != http.StatusUnauthorized {
		t.Fatalf("POST without JWT expected 401, got %d", post.StatusCode)
	}
}

func TestListDependencyLinksRequiresRepoIDs(t *testing.T) {
	_, srv := depLinksServer(t)
	ts := startTestServer(t, srv)
	token, _ := auth.GenerateToken(auth.DefaultUserID, auth.DefaultTeamID, "Team Owner")
	resp := authedGet(t, ts.URL+"/api/v1/dependency-links", token)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 without repo_ids, got %d", resp.StatusCode)
	}
}
