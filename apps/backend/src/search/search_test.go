package search

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ramin1886/git-interactive-history/backend/src/gitengine"
)

// fakeMeili captures the requests the client makes and returns canned hits.
type fakeMeili struct {
	server      *httptest.Server
	indexedDocs []SearchHit
	lastFilter  string
	hits        []SearchHit
}

func newFakeMeili(t *testing.T) *fakeMeili {
	t.Helper()
	f := &fakeMeili{}
	mux := http.NewServeMux()
	mux.HandleFunc("/indexes/commits/settings", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"taskUid":1}`))
	})
	mux.HandleFunc("/indexes/commits/documents", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &f.indexedDocs)
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"taskUid":2}`))
	})
	mux.HandleFunc("/indexes/commits/search", func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Q      string `json:"q"`
			Filter string `json:"filter"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &payload)
		f.lastFilter = payload.Filter
		json.NewEncoder(w).Encode(searchResponse{Hits: f.hits})
	})
	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	return f
}

func sampleNodes() []gitengine.CommitNode {
	return []gitengine.CommitNode{
		{Hash: "1_aaa", ShortHash: "aaa", Author: "Alice", Message: "fix bug", RepoID: "1", Tag: "v1", Kind: "commit", Count: 1},
		{Hash: "1_bbb", ShortHash: "bbb", Author: "Bob", Message: "add feature", RepoID: "1", Kind: "commit", Count: 1},
		// Aggregate nodes must be skipped during indexing.
		{Hash: "agg_1_aaa_bbb", ShortHash: "+2", Message: "2 commits collapsed", RepoID: "1", Kind: "aggregate", Count: 2},
	}
}

func TestIndexCommitsUpsertsRealCommitsOnly(t *testing.T) {
	f := newFakeMeili(t)
	c := NewClientWithConfig(f.server.URL, "key")

	if err := c.IndexCommits(context.Background(), "1", sampleNodes()); err != nil {
		t.Fatalf("IndexCommits: %v", err)
	}
	if len(f.indexedDocs) != 2 {
		t.Fatalf("expected 2 indexed docs (aggregate skipped), got %d", len(f.indexedDocs))
	}
	if f.indexedDocs[0].Hash != "1_aaa" || f.indexedDocs[0].Tag != "v1" {
		t.Fatalf("first indexed doc wrong: %+v", f.indexedDocs[0])
	}
}

func TestIndexCommitsEmptyIsNoop(t *testing.T) {
	f := newFakeMeili(t)
	c := NewClientWithConfig(f.server.URL, "key")
	if err := c.IndexCommits(context.Background(), "1", nil); err != nil {
		t.Fatalf("IndexCommits(nil): %v", err)
	}
	if len(f.indexedDocs) != 0 {
		t.Fatal("no docs should be indexed for empty input")
	}
}

func TestSearchReturnsHitsAndScopesByRepo(t *testing.T) {
	f := newFakeMeili(t)
	f.hits = []SearchHit{{Hash: "1_aaa", ShortHash: "aaa", Author: "Alice", Message: "fix bug", RepoID: "1"}}
	c := NewClientWithConfig(f.server.URL, "key")

	hits, err := c.Search(context.Background(), "bug", []string{"1", "2"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 1 || hits[0].Hash != "1_aaa" {
		t.Fatalf("unexpected hits: %+v", hits)
	}
	if !strings.Contains(f.lastFilter, `repo_id = "1"`) || !strings.Contains(f.lastFilter, `repo_id = "2"`) {
		t.Fatalf("filter should OR both repos, got %q", f.lastFilter)
	}
}

func TestSearchNoRepoFilterWhenEmpty(t *testing.T) {
	f := newFakeMeili(t)
	c := NewClientWithConfig(f.server.URL, "key")
	if _, err := c.Search(context.Background(), "bug", nil); err != nil {
		t.Fatalf("Search: %v", err)
	}
	if f.lastFilter != "" {
		t.Fatalf("expected no filter, got %q", f.lastFilter)
	}
}

func TestIndexCommitsDegradesWhenMeiliDown(t *testing.T) {
	// Point at an address that refuses connections; IndexCommits logs+nil.
	c := NewClientWithConfig("http://127.0.0.1:1", "key")
	if err := c.IndexCommits(context.Background(), "1", sampleNodes()); err != nil {
		t.Fatalf("IndexCommits should degrade gracefully, got %v", err)
	}
}

func TestSearchErrorsWhenMeiliDown(t *testing.T) {
	c := NewClientWithConfig("http://127.0.0.1:1", "key")
	if _, err := c.Search(context.Background(), "bug", nil); err == nil {
		t.Fatal("Search should return an error when Meili is unreachable")
	}
}

func TestSearchErrorsOnBadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := NewClientWithConfig(srv.URL, "key")
	if _, err := c.Search(context.Background(), "bug", nil); err == nil {
		t.Fatal("Search should error on non-2xx status")
	}
}
