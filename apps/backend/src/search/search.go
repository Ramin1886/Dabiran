// Package search is a thin client over the Meilisearch HTTP API used to index
// and full-text query commit messages. It degrades gracefully when Meili is
// unreachable: indexing logs and returns nil (best-effort), while querying
// returns an error the API layer maps to 503.
package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ramin1886/git-interactive-history/backend/src/gitengine"
)

// indexName is the single Meilisearch index holding commits across all repos;
// per-repo scoping is done with a repo_id filter at query time.
const indexName = "commits"

// SearchHit is one full-text match returned by Search. The JSON tags are the
// wire contract with packages/shared-types/src/index.ts (SearchHit) and the
// document shape stored in Meilisearch — keep them snake_case and in sync.
type SearchHit struct {
	Hash      string `json:"hash"`
	ShortHash string `json:"short_hash"`
	Author    string `json:"author"`
	Message   string `json:"message"`
	RepoID    string `json:"repo_id"`
	Tag       string `json:"tag"`
}

// Client talks to one Meilisearch instance. Construct it with NewClient.
type Client struct {
	host       string
	apiKey     string
	httpClient *http.Client
}

// NewClient builds a Meilisearch client from the MEILI_URL (default
// http://localhost:7700) and MEILI_MASTER_KEY environment variables.
func NewClient() *Client {
	host := os.Getenv("MEILI_URL")
	if host == "" {
		host = "http://localhost:7700"
	}
	return NewClientWithConfig(host, os.Getenv("MEILI_MASTER_KEY"))
}

// NewClientWithConfig builds a client against an explicit host and key. Tests
// point host at an httptest server so go test never needs a real Meili.
func NewClientWithConfig(host, apiKey string) *Client {
	return &Client{
		host:       strings.TrimRight(host, "/"),
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

// newRequest builds an authenticated JSON request to the Meili API.
func (c *Client) newRequest(ctx context.Context, method, path string, body []byte) (*http.Request, error) {
	var r *http.Request
	var err error
	if body != nil {
		r, err = http.NewRequestWithContext(ctx, method, c.host+path, bytes.NewReader(body))
	} else {
		r, err = http.NewRequestWithContext(ctx, method, c.host+path, nil)
	}
	if err != nil {
		return nil, err
	}
	r.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		r.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	return r, nil
}

// IndexCommits upserts one Meili document per node (id = hash, fields
// hash/short_hash/author/message/repo_id/tag) and ensures repo_id is a
// filterable attribute. It is best-effort: when Meili is unreachable it logs a
// warning and returns nil so topology serving and webhook sync never fail on a
// missing search backend.
func (c *Client) IndexCommits(ctx context.Context, repoID string, nodes []gitengine.CommitNode) error {
	if len(nodes) == 0 {
		return nil
	}

	// Make repo_id filterable so Search can scope by repository. Best-effort;
	// ignore the (idempotent) settings response.
	if req, err := c.newRequest(ctx, http.MethodPatch, "/indexes/"+indexName+"/settings",
		[]byte(`{"filterableAttributes":["repo_id"]}`)); err == nil {
		if resp, derr := c.httpClient.Do(req); derr == nil {
			resp.Body.Close()
		}
	}

	docs := make([]SearchHit, 0, len(nodes))
	for _, n := range nodes {
		// Skip synthetic aggregate nodes: they are not real searchable commits.
		if n.Kind == "aggregate" {
			continue
		}
		docs = append(docs, SearchHit{
			Hash:      n.Hash,
			ShortHash: n.ShortHash,
			Author:    n.Author,
			Message:   n.Message,
			RepoID:    n.RepoID,
			Tag:       n.Tag,
		})
	}
	if len(docs) == 0 {
		return nil
	}

	body, err := json.Marshal(docs)
	if err != nil {
		return err
	}
	// Meili infers the primary key "hash" from the first index; pass it
	// explicitly so a fresh index resolves the id field deterministically.
	req, err := c.newRequest(ctx, http.MethodPost, "/indexes/"+indexName+"/documents?primaryKey=hash", body)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("search: IndexCommits for repo %s skipped, Meili unreachable: %v", repoID, err)
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		log.Printf("search: IndexCommits for repo %s got status %d from Meili", repoID, resp.StatusCode)
	}
	return nil
}

// searchResponse models the subset of the Meili /search response we read.
type searchResponse struct {
	Hits []SearchHit `json:"hits"`
}

// Search runs a full-text query q, optionally scoped to repoIDs (OR-ed
// repo_id filter), and returns the matching hits. Unlike IndexCommits it does
// NOT degrade silently: when Meili is unreachable or errors it returns an
// error so the handler can surface 503 to the caller.
func (c *Client) Search(ctx context.Context, q string, repoIDs []string) ([]SearchHit, error) {
	payload := map[string]any{"q": q, "limit": 100}
	if len(repoIDs) > 0 {
		clauses := make([]string, 0, len(repoIDs))
		for _, id := range repoIDs {
			clauses = append(clauses, fmt.Sprintf("repo_id = %q", id))
		}
		payload["filter"] = strings.Join(clauses, " OR ")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := c.newRequest(ctx, http.MethodPost, "/indexes/"+indexName+"/search", body)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("meilisearch unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("meilisearch search failed with status %d", resp.StatusCode)
	}
	var out searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode meilisearch response: %w", err)
	}
	if out.Hits == nil {
		out.Hits = make([]SearchHit, 0)
	}
	return out.Hits, nil
}
