package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

// stateCookieName holds the CSRF state between the login redirect and the
// provider callback.
const stateCookieName = "oauth_state"

// defaultOrgEnv names the environment variable supplying the fallback org
// (team) for users who belong to no GitHub organization.
const defaultOrgEnv = "OAUTH_DEFAULT_ORG"

// fallbackDefaultOrg is the team/org name used when a user has no orgs and
// OAUTH_DEFAULT_ORG is unset (see apps/backend/.env.example).
const fallbackDefaultOrg = "default-team"

// OAuthHandler serves the GitHub OAuth2 endpoints. It holds the database pool
// (used to persist the resolved identity) and a GitHubClient (injectable for
// tests). When DB is nil — local dev without Postgres — the callback falls
// back to the single-tenant default identity so development still works.
type OAuthHandler struct {
	DB     *pgxpool.Pool
	GitHub GitHubClient
}

// NewOAuthHandler constructs an OAuthHandler over pool (which may be nil) and
// the production GitHub client.
func NewOAuthHandler(pool *pgxpool.Pool) *OAuthHandler {
	return &OAuthHandler{DB: pool, GitHub: NewGitHubClient()}
}

// GetOAuthConfig builds the GitHub OAuth2 configuration from the
// GITHUB_CLIENT_ID, GITHUB_CLIENT_SECRET, and OAUTH_REDIRECT_URL environment
// variables (see apps/backend/.env.example).
func GetOAuthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     os.Getenv("GITHUB_CLIENT_ID"),
		ClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
		Scopes:       []string{"read:org", "repo"},
		Endpoint:     github.Endpoint,
		RedirectURL:  os.Getenv("OAUTH_REDIRECT_URL"),
	}
}

// generateState returns a 32-byte crypto/rand CSRF token, base64url-encoded.
func generateState() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// HandleLogin starts the GitHub OAuth2 flow (GET /api/v1/auth/github/login):
// it generates a random CSRF state, stores it in a short-lived HttpOnly
// cookie, and issues a 307 redirect to GitHub's authorize URL carrying the
// same state.
func (h *OAuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	state, err := generateState()
	if err != nil {
		http.Error(w, "failed to generate oauth state", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    state,
		Path:     "/",
		MaxAge:   600, // the round-trip to GitHub should take seconds, not minutes
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, GetOAuthConfig().AuthCodeURL(state), http.StatusTemporaryRedirect)
}

// HandleCallback completes the OAuth2 flow (GET /api/v1/auth/github/callback):
// it verifies the CSRF state against the login cookie (401 on mismatch per
// docs/apis_doc.md), exchanges the authorization code for a GitHub token,
// resolves the caller's persistent identity (see resolveIdentity), and
// responds with {"access_token": <internal JWT>, "role": <role>}.
func (h *OAuthHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(stateCookieName)
	if err != nil || cookie.Value == "" || r.FormValue("state") != cookie.Value {
		http.Error(w, "invalid oauth state", http.StatusUnauthorized)
		return
	}
	// The state is single-use: expire the cookie immediately.
	http.SetCookie(w, &http.Cookie{Name: stateCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true})

	conf := GetOAuthConfig()
	token, err := conf.Exchange(r.Context(), r.FormValue("code"))
	if err != nil {
		http.Error(w, "oauth code exchange failed", http.StatusUnauthorized)
		return
	}
	if !token.Valid() {
		http.Error(w, "oauth token invalid", http.StatusUnauthorized)
		return
	}

	userID, teamID, role, err := h.resolveIdentity(r.Context(), token.AccessToken)
	if err != nil {
		http.Error(w, "failed to resolve identity", http.StatusInternalServerError)
		return
	}

	systemToken, err := GenerateToken(userID, teamID, role)
	if err != nil {
		http.Error(w, "failed to issue session token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"access_token": systemToken, "role": role})
}

// resolveIdentity maps a GitHub OAuth access token onto a persistent
// (userID, teamID, role) identity and returns it for embedding in the JWT.
//
// Mapping policy:
//   - When DB is nil (local dev without Postgres), no persistence is possible,
//     so the single-tenant default identity is returned unchanged.
//   - Otherwise the user is fetched from GitHub. The users row is keyed by
//     email; when GitHub hides the email a synthetic
//     "<login>@users.noreply.github.com" address is used instead.
//   - The PRIMARY org is the first org GitHub returns, or OAUTH_DEFAULT_ORG
//     (falling back to fallbackDefaultOrg) when the user has no orgs. A teams
//     row named after the primary org is upserted with this user as owner, and
//     a team_memberships row binds the user to it.
//   - Role is "Team Owner" when the user is an org admin OR owns the team,
//     else "Team Member". Org-admin lookup is best-effort: errors mean member.
func (h *OAuthHandler) resolveIdentity(ctx context.Context, ghToken string) (userID int, teamID int, role string, err error) {
	if h.DB == nil {
		return DefaultUserID, DefaultTeamID, "Team Owner", nil
	}

	profile, err := h.GitHub.Profile(ctx, ghToken)
	if err != nil {
		return 0, 0, "", fmt.Errorf("fetch github profile: %w", err)
	}
	email := profile.Email
	if email == "" {
		email = profile.Login + "@users.noreply.github.com"
	}

	orgs, err := h.GitHub.Orgs(ctx, ghToken)
	if err != nil {
		return 0, 0, "", fmt.Errorf("fetch github orgs: %w", err)
	}
	primaryOrg := defaultOrg()
	if len(orgs) > 0 {
		primaryOrg = orgs[0]
	}

	// Upsert the user keyed by email; ON CONFLICT keeps the row stable across
	// logins while refreshing the display name.
	if err = h.DB.QueryRow(ctx,
		`INSERT INTO users (email, name) VALUES ($1, $2)
		 ON CONFLICT (email) DO UPDATE SET name = EXCLUDED.name
		 RETURNING id`,
		email, profile.Name).Scan(&userID); err != nil {
		return 0, 0, "", fmt.Errorf("upsert user: %w", err)
	}

	// Upsert the team named after the primary org. teams.name is not unique in
	// the schema, so we look up an existing row first and only insert when none
	// exists, recording this user as owner of any newly created team.
	var ownerID int
	if err = h.DB.QueryRow(ctx,
		`INSERT INTO teams (name, owner_id) VALUES ($1, $2)
		 ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
		 RETURNING id, owner_id`,
		primaryOrg, userID).Scan(&teamID, &ownerID); err != nil {
		return 0, 0, "", fmt.Errorf("upsert team: %w", err)
	}

	isAdmin, adminErr := h.GitHub.IsOrgAdmin(ctx, ghToken, primaryOrg)
	if adminErr != nil {
		isAdmin = false // best-effort: treat lookup failures as member
	}
	role = "Team Member"
	if isAdmin || ownerID == userID {
		role = "Team Owner"
	}

	// Upsert the membership with the resolved role.
	if _, err = h.DB.Exec(ctx,
		`INSERT INTO team_memberships (team_id, user_id, role) VALUES ($1, $2, $3)
		 ON CONFLICT (team_id, user_id) DO UPDATE SET role = EXCLUDED.role`,
		teamID, userID, role); err != nil {
		return 0, 0, "", fmt.Errorf("upsert membership: %w", err)
	}

	return userID, teamID, role, nil
}

// defaultOrg returns the fallback org name for users with no GitHub orgs,
// preferring OAUTH_DEFAULT_ORG over the built-in fallbackDefaultOrg.
func defaultOrg() string {
	if v := os.Getenv(defaultOrgEnv); v != "" {
		return v
	}
	return fallbackDefaultOrg
}
