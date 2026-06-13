package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ramin1886/git-interactive-history/backend/src/db"
)

// stateCookie extracts the oauth state cookie from a recorded response.
func stateCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == stateCookieName {
			return c
		}
	}
	return nil
}

// fakeGitHub is an injectable GitHubClient that returns canned data so tests
// never hit the network.
type fakeGitHub struct {
	profile GitHubProfile
	orgs    []string
	admin   bool
	adminErr error
}

func (f *fakeGitHub) Profile(context.Context, string) (GitHubProfile, error) { return f.profile, nil }
func (f *fakeGitHub) Orgs(context.Context, string) ([]string, error)         { return f.orgs, nil }
func (f *fakeGitHub) IsOrgAdmin(context.Context, string, string) (bool, error) {
	return f.admin, f.adminErr
}

// testPool opens the live database pool or skips when DATABASE_URL is unset.
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set; skipping live database test")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	if err := db.Migrate(context.Background(), pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestHandleLoginSetsStateCookieAndRedirects(t *testing.T) {
	h := &OAuthHandler{}
	rec := httptest.NewRecorder()
	h.HandleLogin(rec, httptest.NewRequest(http.MethodGet, "/api/v1/auth/github/login", nil))

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected 307, got %d", rec.Code)
	}
	cookie := stateCookie(t, rec)
	if cookie == nil || cookie.Value == "" {
		t.Fatal("state cookie not set")
	}
	if !cookie.HttpOnly {
		t.Fatal("state cookie must be HttpOnly")
	}

	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("invalid redirect location: %v", err)
	}
	if got := loc.Query().Get("state"); got != cookie.Value {
		t.Fatalf("redirect state %q does not match cookie %q", got, cookie.Value)
	}
}

func TestHandleLoginStateIsRandom(t *testing.T) {
	h := &OAuthHandler{}
	values := map[string]bool{}
	for i := 0; i < 3; i++ {
		rec := httptest.NewRecorder()
		h.HandleLogin(rec, httptest.NewRequest(http.MethodGet, "/login", nil))
		c := stateCookie(t, rec)
		if c == nil {
			t.Fatal("state cookie not set")
		}
		if values[c.Value] {
			t.Fatal("state value repeated across logins")
		}
		values[c.Value] = true
	}
}

func TestHandleCallbackRejectsMismatchedState(t *testing.T) {
	h := &OAuthHandler{}
	req := httptest.NewRequest(http.MethodGet, "/callback?state=forged&code=abc", nil)
	req.AddCookie(&http.Cookie{Name: stateCookieName, Value: "genuine"})
	rec := httptest.NewRecorder()
	h.HandleCallback(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for mismatched state, got %d", rec.Code)
	}
}

func TestHandleCallbackRejectsMissingStateCookie(t *testing.T) {
	h := &OAuthHandler{}
	req := httptest.NewRequest(http.MethodGet, "/callback?state=whatever&code=abc", nil)
	rec := httptest.NewRecorder()
	h.HandleCallback(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing state cookie, got %d", rec.Code)
	}
}

func TestResolveIdentityNilPoolFallsBackToDefault(t *testing.T) {
	h := &OAuthHandler{DB: nil, GitHub: &fakeGitHub{}}
	userID, teamID, role, err := h.resolveIdentity(context.Background(), "tok")
	if err != nil {
		t.Fatalf("resolveIdentity: %v", err)
	}
	if userID != DefaultUserID || teamID != DefaultTeamID || role != "Team Owner" {
		t.Fatalf("nil-pool fallback wrong: %d/%d/%s", userID, teamID, role)
	}
}

// cleanupIdentity removes the rows created for a test user/team so reruns stay
// deterministic. Order respects foreign keys.
func cleanupIdentity(t *testing.T, pool *pgxpool.Pool, email, org string) {
	t.Helper()
	ctx := context.Background()
	pool.Exec(ctx, `DELETE FROM team_memberships WHERE user_id IN (SELECT id FROM users WHERE email=$1)`, email)
	pool.Exec(ctx, `DELETE FROM team_memberships WHERE team_id IN (SELECT id FROM teams WHERE name=$1)`, org)
	pool.Exec(ctx, `DELETE FROM teams WHERE name=$1`, org)
	pool.Exec(ctx, `DELETE FROM users WHERE email=$1`, email)
}

func TestResolveIdentityNewUserAndTeam(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	email := "gh-newuser-test@example.com"
	org := "gh-neworg-test"
	cleanupIdentity(t, pool, email, org)
	t.Cleanup(func() { cleanupIdentity(t, pool, email, org) })

	h := &OAuthHandler{DB: pool, GitHub: &fakeGitHub{
		profile: GitHubProfile{Login: "newuser", Name: "New User", Email: email},
		orgs:    []string{org},
		admin:   false,
	}}
	userID, teamID, role, err := h.resolveIdentity(ctx, "tok")
	if err != nil {
		t.Fatalf("resolveIdentity: %v", err)
	}
	if userID == 0 || teamID == 0 {
		t.Fatalf("expected non-zero ids, got %d/%d", userID, teamID)
	}
	// New user owns the freshly created team, so they are Team Owner even
	// though they are not an org admin.
	if role != "Team Owner" {
		t.Fatalf("team creator should be Team Owner, got %q", role)
	}
	var name string
	if err := pool.QueryRow(ctx, `SELECT name FROM teams WHERE id=$1`, teamID).Scan(&name); err != nil {
		t.Fatalf("team lookup: %v", err)
	}
	if name != org {
		t.Fatalf("team named %q want %q", name, org)
	}
	var mRole string
	if err := pool.QueryRow(ctx, `SELECT role FROM team_memberships WHERE team_id=$1 AND user_id=$2`, teamID, userID).Scan(&mRole); err != nil {
		t.Fatalf("membership lookup: %v", err)
	}
	if mRole != "Team Owner" {
		t.Fatalf("membership role %q want Team Owner", mRole)
	}
}

func TestResolveIdentityOrgAdminIsTeamOwnerNonOwner(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	ownerEmail := "gh-owner-test@example.com"
	memberEmail := "gh-admin-test@example.com"
	org := "gh-sharedorg-test"
	cleanupIdentity(t, pool, ownerEmail, org)
	cleanupIdentity(t, pool, memberEmail, org)
	t.Cleanup(func() {
		cleanupIdentity(t, pool, memberEmail, org)
		cleanupIdentity(t, pool, ownerEmail, org)
	})

	// First login creates the org/team owned by the owner user.
	owner := &OAuthHandler{DB: pool, GitHub: &fakeGitHub{
		profile: GitHubProfile{Login: "owner", Email: ownerEmail},
		orgs:    []string{org},
	}}
	if _, _, _, err := owner.resolveIdentity(ctx, "tok"); err != nil {
		t.Fatalf("owner resolveIdentity: %v", err)
	}

	// Second user is not the team owner; they become Team Owner only via admin.
	adminUser := &OAuthHandler{DB: pool, GitHub: &fakeGitHub{
		profile: GitHubProfile{Login: "adminuser", Email: memberEmail},
		orgs:    []string{org},
		admin:   true,
	}}
	_, _, role, err := adminUser.resolveIdentity(ctx, "tok")
	if err != nil {
		t.Fatalf("admin resolveIdentity: %v", err)
	}
	if role != "Team Owner" {
		t.Fatalf("org admin should be Team Owner, got %q", role)
	}

	// A plain member of an existing team they do not own is Team Member.
	member := &OAuthHandler{DB: pool, GitHub: &fakeGitHub{
		profile: GitHubProfile{Login: "memberuser", Email: "gh-plainmember-test@example.com"},
		orgs:    []string{org},
		admin:   false,
	}}
	t.Cleanup(func() { cleanupIdentity(t, pool, "gh-plainmember-test@example.com", org) })
	_, _, mrole, err := member.resolveIdentity(ctx, "tok")
	if err != nil {
		t.Fatalf("member resolveIdentity: %v", err)
	}
	if mrole != "Team Member" {
		t.Fatalf("non-owner non-admin should be Team Member, got %q", mrole)
	}
}

func TestResolveIdentityNoEmailFallback(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	syntheticEmail := "noemailuser@users.noreply.github.com"
	org := "gh-noemailorg-test"
	cleanupIdentity(t, pool, syntheticEmail, org)
	t.Cleanup(func() { cleanupIdentity(t, pool, syntheticEmail, org) })

	h := &OAuthHandler{DB: pool, GitHub: &fakeGitHub{
		profile: GitHubProfile{Login: "noemailuser", Email: ""},
		orgs:    []string{org},
	}}
	userID, _, _, err := h.resolveIdentity(ctx, "tok")
	if err != nil {
		t.Fatalf("resolveIdentity: %v", err)
	}
	var email string
	if err := pool.QueryRow(ctx, `SELECT email FROM users WHERE id=$1`, userID).Scan(&email); err != nil {
		t.Fatalf("user lookup: %v", err)
	}
	if email != syntheticEmail {
		t.Fatalf("synthetic email %q want %q", email, syntheticEmail)
	}
}

func TestResolveIdentityNoOrgsUsesDefault(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	email := "gh-noorgs-test@example.com"
	t.Setenv(defaultOrgEnv, "gh-configured-default-test")
	org := "gh-configured-default-test"
	cleanupIdentity(t, pool, email, org)
	t.Cleanup(func() { cleanupIdentity(t, pool, email, org) })

	h := &OAuthHandler{DB: pool, GitHub: &fakeGitHub{
		profile: GitHubProfile{Login: "noorgs", Email: email},
		orgs:    nil,
	}}
	_, teamID, _, err := h.resolveIdentity(ctx, "tok")
	if err != nil {
		t.Fatalf("resolveIdentity: %v", err)
	}
	var name string
	if err := pool.QueryRow(ctx, `SELECT name FROM teams WHERE id=$1`, teamID).Scan(&name); err != nil {
		t.Fatalf("team lookup: %v", err)
	}
	if name != org {
		t.Fatalf("default-org team %q want %q", name, org)
	}
}
