package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// GitHubProfile is the subset of GET https://api.github.com/user consumed by
// the identity-mapping policy in HandleCallback.
type GitHubProfile struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// GitHubClient abstracts the GitHub REST calls needed to map an OAuth login
// onto a user/team identity. It is an interface so tests can inject a fake and
// never touch the network.
type GitHubClient interface {
	// Profile fetches the authenticated user (GET /user).
	Profile(ctx context.Context, token string) (GitHubProfile, error)
	// Orgs returns the login names of the user's organizations (GET /user/orgs),
	// in GitHub's response order; the first is treated as the primary org.
	Orgs(ctx context.Context, token string) ([]string, error)
	// IsOrgAdmin reports whether the user is an admin of org
	// (GET /user/memberships/orgs/{org}). It is best-effort: any error is
	// reported as false (member) by callers.
	IsOrgAdmin(ctx context.Context, token, org string) (bool, error)
}

// httpGitHubClient is the production GitHubClient backed by api.github.com.
type httpGitHubClient struct {
	http    *http.Client
	baseURL string
}

// NewGitHubClient returns the default GitHubClient targeting api.github.com
// with a 10-second per-request timeout.
func NewGitHubClient() GitHubClient {
	return &httpGitHubClient{
		http:    &http.Client{Timeout: 10 * time.Second},
		baseURL: "https://api.github.com",
	}
}

// getJSON issues an authenticated GET against path and decodes the JSON body
// into out, returning an error for non-2xx responses.
func (c *httpGitHubClient) getJSON(ctx context.Context, token, path string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("github GET %s: status %d", path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// Profile fetches the authenticated user from GET /user.
func (c *httpGitHubClient) Profile(ctx context.Context, token string) (GitHubProfile, error) {
	var p GitHubProfile
	err := c.getJSON(ctx, token, "/user", &p)
	return p, err
}

// Orgs lists the user's organization logins from GET /user/orgs.
func (c *httpGitHubClient) Orgs(ctx context.Context, token string) ([]string, error) {
	var raw []struct {
		Login string `json:"login"`
	}
	if err := c.getJSON(ctx, token, "/user/orgs", &raw); err != nil {
		return nil, err
	}
	logins := make([]string, 0, len(raw))
	for _, o := range raw {
		logins = append(logins, o.Login)
	}
	return logins, nil
}

// IsOrgAdmin reports the user's role in org via
// GET /user/memberships/orgs/{org}, returning true only for role "admin".
func (c *httpGitHubClient) IsOrgAdmin(ctx context.Context, token, org string) (bool, error) {
	var m struct {
		Role string `json:"role"`
	}
	if err := c.getJSON(ctx, token, "/user/memberships/orgs/"+org, &m); err != nil {
		return false, err
	}
	return m.Role == "admin", nil
}
