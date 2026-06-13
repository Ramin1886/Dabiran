package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

// stateCookieName holds the CSRF state between the login redirect and the
// provider callback.
const stateCookieName = "oauth_state"

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
func HandleLogin(w http.ResponseWriter, r *http.Request) {
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
// docs/apis_doc.md), exchanges the authorization code for a GitHub token, and
// responds with {"access_token": <internal JWT>, "role": <role>}.
func HandleCallback(w http.ResponseWriter, r *http.Request) {
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

	// TODO: fetch the real GitHub profile (GET https://api.github.com/user
	// with token) and map it onto a users row. That requires user persistence
	// and an org->team mapping policy that do not exist yet, so we issue the
	// single-tenant default identity for now.
	const role = "Team Owner"
	systemToken, err := GenerateToken(DefaultUserID, DefaultTeamID, role)
	if err != nil {
		http.Error(w, "failed to issue session token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"access_token": systemToken, "role": role})
}
