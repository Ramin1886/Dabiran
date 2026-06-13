package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/go-git/go-git/v5"
	"github.com/ramin1886/git-interactive-history/backend/src/crypto"
	"github.com/ramin1886/git-interactive-history/backend/src/gitengine"
)

// repoSyncer is the slice of the git engine the webhook handler needs. It is
// an interface so tests can stub the fetch instead of hitting the network
// (gitengine.GitEngine satisfies it).
type repoSyncer interface {
	EnsureRepository(ctx context.Context, repoID int, url, authType, authSecret string) (*git.Repository, error)
}

// githubPushPayload is the subset of the GitHub push webhook body we read.
type githubPushPayload struct {
	Repository struct {
		FullName string `json:"full_name"`
		CloneURL string `json:"clone_url"`
		HTMLURL  string `json:"html_url"`
		SSHURL   string `json:"ssh_url"`
	} `json:"repository"`
}

// verifySignature reports whether sigHeader (the X-Hub-Signature-256 value,
// "sha256=<hex>") is a valid HMAC-SHA256 of body under secret, using a
// constant-time comparison.
func verifySignature(secret string, body []byte, sigHeader string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(sigHeader))
}

// HandleGitHubWebhook handles POST /api/v1/webhooks/github. It is NOT behind
// RequireAuth — GitHub authenticates the request by signing the body.
//
// Security: the X-Hub-Signature-256 HMAC-SHA256 over the raw body is verified
// in constant time against env GITHUB_WEBHOOK_SECRET (401 on mismatch). When
// that env var is unset the handler logs a warning and SKIPS verification so
// local development works without a configured secret — production MUST set
// GITHUB_WEBHOOK_SECRET (documented in .env.example).
//
// Routing: on X-GitHub-Event: push it resolves the repositories row matching
// the payload repository URL, then triggers a background fetch (responding 202
// immediately) and re-indexes the refreshed commits. All other events → 204.
func (s *APIServer) HandleGitHubWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	secret := os.Getenv("GITHUB_WEBHOOK_SECRET")
	if secret == "" {
		log.Println("WARNING: GITHUB_WEBHOOK_SECRET unset; skipping webhook signature verification (dev mode)")
	} else if !verifySignature(secret, body, r.Header.Get("X-Hub-Signature-256")) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	if r.Header.Get("X-GitHub-Event") != "push" {
		// Acknowledge non-push events without further work.
		w.WriteHeader(http.StatusNoContent)
		return
	}

	var payload githubPushPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	// Resolve the tracked repository by any of the URLs GitHub may report.
	candidates := []string{
		payload.Repository.CloneURL,
		payload.Repository.HTMLURL,
		payload.Repository.SSHURL,
	}
	repoID, url, authType, secretCred, found := s.lookupRepositoryByURL(r.Context(), candidates)
	if !found {
		// Unknown repository: acknowledge gracefully (nothing to sync).
		log.Printf("webhook: push for untracked repository %q ignored", payload.Repository.FullName)
		w.WriteHeader(http.StatusAccepted)
		return
	}

	// Fetch new commits asynchronously, then re-index. Respond 202 now.
	go func() {
		ctx := context.Background()
		repo, err := s.syncer().EnsureRepository(ctx, repoID, url, authType, secretCred)
		if err != nil {
			log.Printf("webhook: fetch for repo %d failed: %v", repoID, err)
			return
		}
		if s.Search == nil {
			return
		}
		idStr := strconv.Itoa(repoID)
		nodes, err := gitengine.ExtractUnifiedTopology(map[string]*git.Repository{idStr: repo})
		if err != nil {
			log.Printf("webhook: topology extract for repo %d failed: %v", repoID, err)
			return
		}
		if err := s.Search.IndexCommits(ctx, idStr, nodes); err != nil {
			log.Printf("webhook: reindex for repo %d failed: %v", repoID, err)
		}
	}()

	w.WriteHeader(http.StatusAccepted)
}

// syncer returns the configured repo syncer, defaulting to the git engine.
func (s *APIServer) syncer() repoSyncer {
	if s.RepoSyncer != nil {
		return s.RepoSyncer
	}
	return s.Engine
}

// lookupRepositoryByURL finds the repositories row whose url matches any of
// candidates, decrypting its stored credential. It returns found=false when
// the DB is nil or no row matches. Credential decryption errors degrade to an
// anonymous fetch (empty secret) rather than failing the webhook.
func (s *APIServer) lookupRepositoryByURL(ctx context.Context, candidates []string) (repoID int, url, authType, secret string, found bool) {
	if s.DB == nil {
		return 0, "", "", "", false
	}
	urls := make([]string, 0, len(candidates))
	for _, c := range candidates {
		if c != "" {
			urls = append(urls, c)
		}
	}
	if len(urls) == 0 {
		return 0, "", "", "", false
	}
	var encrypted string
	row := s.DB.QueryRow(ctx,
		"SELECT id, url, auth_type, encrypted_credential FROM repositories WHERE url = ANY($1) LIMIT 1", urls)
	if err := row.Scan(&repoID, &url, &authType, &encrypted); err != nil {
		return 0, "", "", "", false
	}
	if encrypted != "" {
		if key, kerr := crypto.MasterKey(); kerr == nil {
			if plain, derr := crypto.Decrypt(encrypted, key); derr == nil {
				secret = string(plain)
			} else {
				log.Printf("webhook: credential decrypt for repo %d failed, fetching anonymously: %v", repoID, derr)
			}
		}
	}
	return repoID, url, authType, secret, true
}
