package gitengine

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// testBaseTime is the author timestamp of the first commit in the fixture.
var testBaseTime = time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

// fixtureHashes holds the commit SHAs produced by buildSourceRepo, in
// chronological order (c1 -> c2 on main, c3 branched off c1).
type fixtureHashes struct {
	c1, c2, c3 plumbing.Hash
}

// testSignature returns a deterministic author signature offset seconds
// after testBaseTime.
func testSignature(offset time.Duration) *object.Signature {
	return &object.Signature{Name: "Alice", Email: "alice@example.com", When: testBaseTime.Add(offset)}
}

// buildSourceRepo creates a temp non-bare repository with three commits
// (c1 <- c2 on the default branch, c3 on a "feature" branch forked from c1),
// a lightweight tag "light-tag" on c2 and an annotated tag "v1.0.0" on c3.
// It returns the repo directory and the commit hashes.
func buildSourceRepo(t *testing.T) (string, fixtureHashes) {
	t.Helper()
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("PlainInit: %v", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}

	commit := func(file, content, msg string, offset time.Duration) plumbing.Hash {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", file, err)
		}
		if _, err := wt.Add(file); err != nil {
			t.Fatalf("add %s: %v", file, err)
		}
		sig := testSignature(offset)
		h, err := wt.Commit(msg, &git.CommitOptions{Author: sig, Committer: sig})
		if err != nil {
			t.Fatalf("commit %s: %v", msg, err)
		}
		return h
	}

	var hashes fixtureHashes
	hashes.c1 = commit("a.txt", "one", "c1", 0)
	hashes.c2 = commit("a.txt", "two", "c2", 100*time.Second)

	// Fork "feature" off c1 and add c3 there.
	if err := wt.Checkout(&git.CheckoutOptions{
		Hash:   hashes.c1,
		Branch: plumbing.NewBranchReferenceName("feature"),
		Create: true,
	}); err != nil {
		t.Fatalf("checkout feature: %v", err)
	}
	hashes.c3 = commit("b.txt", "three", "c3", 200*time.Second)

	// Lightweight tag on c2, annotated tag on c3.
	if _, err := repo.CreateTag("light-tag", hashes.c2, nil); err != nil {
		t.Fatalf("create lightweight tag: %v", err)
	}
	if _, err := repo.CreateTag("v1.0.0", hashes.c3, &git.CreateTagOptions{
		Tagger:  testSignature(300 * time.Second),
		Message: "release",
	}); err != nil {
		t.Fatalf("create annotated tag: %v", err)
	}

	return dir, hashes
}

// cloneBare clones srcDir into a bare repository at dest (all branches and
// tags) and returns it.
func cloneBare(t *testing.T, srcDir, dest string) *git.Repository {
	t.Helper()
	bare, err := git.PlainClone(dest, true, &git.CloneOptions{URL: srcDir, Tags: git.AllTags})
	if err != nil {
		t.Fatalf("bare clone: %v", err)
	}
	err = bare.Fetch(&git.FetchOptions{
		RefSpecs: []config.RefSpec{"+refs/heads/*:refs/heads/*", "+refs/tags/*:refs/tags/*"},
		Tags:     git.AllTags,
		Force:    true,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		t.Fatalf("fetch all refs: %v", err)
	}
	return bare
}

// nodeByHash finds the node whose prefixed hash matches repoID_sha.
func nodeByHash(t *testing.T, nodes []CommitNode, repoID string, h plumbing.Hash) CommitNode {
	t.Helper()
	want := repoID + "_" + h.String()
	for _, n := range nodes {
		if n.Hash == want {
			return n
		}
	}
	t.Fatalf("node %s not found", want)
	return CommitNode{}
}

func TestExtractUnifiedTopologyLayoutAndTags(t *testing.T) {
	srcDir, hashes := buildSourceRepo(t)
	bare := cloneBare(t, srcDir, filepath.Join(t.TempDir(), "mock_1.git"))

	nodes, err := ExtractUnifiedTopology(map[string]*git.Repository{"1": bare})
	if err != nil {
		t.Fatalf("ExtractUnifiedTopology: %v", err)
	}
	if len(nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(nodes))
	}

	// Chronological ordering: c1, c2, c3.
	wantOrder := []plumbing.Hash{hashes.c1, hashes.c2, hashes.c3}
	for i, h := range wantOrder {
		if nodes[i].Hash != "1_"+h.String() {
			t.Fatalf("position %d: got %s want %s", i, nodes[i].Hash, "1_"+h.String())
		}
	}

	// x_offset = seconds since origin * 0.05.
	for i, want := range []float64{0, 5, 10} {
		if math.Abs(nodes[i].XOffset-want) > 1e-9 {
			t.Fatalf("node %d x_offset: got %v want %v", i, nodes[i].XOffset, want)
		}
	}

	// Lane assignment: c1 and c2 share lane 0 (c2 takes over its primary
	// parent's lane); c3 forked off c1 whose lane tip is now c2, so it
	// claims the lowest free lane: 1.
	if nodes[0].Lane != 0 || nodes[1].Lane != 0 {
		t.Fatalf("main chain lanes: got %d,%d want 0,0", nodes[0].Lane, nodes[1].Lane)
	}
	if nodes[2].Lane != 1 {
		t.Fatalf("branch lane: got %d want 1", nodes[2].Lane)
	}

	// Parent prefixing.
	c2 := nodeByHash(t, nodes, "1", hashes.c2)
	if len(c2.Parents) != 1 || c2.Parents[0] != "1_"+hashes.c1.String() {
		t.Fatalf("c2 parents: %v", c2.Parents)
	}

	// Tag resolution: lightweight on c2, annotated on c3, none on c1.
	if got := nodeByHash(t, nodes, "1", hashes.c1).Tag; got != "" {
		t.Fatalf("c1 tag: got %q want empty", got)
	}
	if got := c2.Tag; got != "light-tag" {
		t.Fatalf("lightweight tag: got %q want light-tag", got)
	}
	if got := nodeByHash(t, nodes, "1", hashes.c3).Tag; got != "v1.0.0" {
		t.Fatalf("annotated tag: got %q want v1.0.0", got)
	}

	// Misc field sanity.
	if nodes[0].RepoID != "1" || nodes[0].Author != "Alice" || !strings.HasPrefix(hashes.c1.String(), nodes[0].ShortHash) {
		t.Fatalf("field sanity failed: %+v", nodes[0])
	}
}

func TestExtractUnifiedTopologyMultiRepoPrefixing(t *testing.T) {
	srcDir, _ := buildSourceRepo(t)
	base := t.TempDir()
	repoA := cloneBare(t, srcDir, filepath.Join(base, "a.git"))
	repoB := cloneBare(t, srcDir, filepath.Join(base, "b.git"))

	nodes, err := ExtractUnifiedTopology(map[string]*git.Repository{"1": repoA, "2": repoB})
	if err != nil {
		t.Fatalf("ExtractUnifiedTopology: %v", err)
	}
	if len(nodes) != 6 {
		t.Fatalf("expected 6 nodes, got %d", len(nodes))
	}

	seen := map[string]bool{}
	for _, n := range nodes {
		if !strings.HasPrefix(n.Hash, n.RepoID+"_") {
			t.Fatalf("hash %s missing repo prefix %s_", n.Hash, n.RepoID)
		}
		for _, p := range n.Parents {
			if !strings.HasPrefix(p, n.RepoID+"_") {
				t.Fatalf("parent %s of %s missing repo prefix", p, n.Hash)
			}
		}
		if seen[n.Hash] {
			t.Fatalf("duplicate prefixed hash %s", n.Hash)
		}
		seen[n.Hash] = true
	}
}

// linearNode builds a chronological commit node n steps after origin with a
// single parent (the previous node's hash) and no tag.
func linearNode(repoID string, idx int, parent string) CommitNode {
	hash := repoID + "_" + strings.Repeat("0", 39) + string(rune('a'+idx))
	parents := []string(nil)
	if parent != "" {
		parents = []string{parent}
	}
	return CommitNode{
		Hash:      hash,
		ShortHash: hash[len(repoID)+1 : len(repoID)+8],
		Author:    "Alice",
		Message:   "commit",
		Date:      testBaseTime.Add(time.Duration(idx) * time.Minute),
		Parents:   parents,
		RepoID:    repoID,
		Kind:      "commit",
		Count:     1,
	}
}

// linearChain builds a straight chain of n linear commits for repo "1".
func linearChain(n int) []CommitNode {
	nodes := make([]CommitNode, 0, n)
	prev := ""
	for i := 0; i < n; i++ {
		node := linearNode("1", i, prev)
		nodes = append(nodes, node)
		prev = node.Hash
	}
	return nodes
}

func TestAggregateLinearRunsNoopWhenWithinBudget(t *testing.T) {
	nodes := linearChain(5)
	cases := []struct {
		name     string
		maxNodes int
	}{
		{"max_nodes absent (zero)", 0},
		{"max_nodes negative", -1},
		{"max_nodes equals count", 5},
		{"max_nodes exceeds count", 10},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := AggregateLinearRuns(append([]CommitNode(nil), nodes...), c.maxNodes)
			if len(out) != 5 {
				t.Fatalf("expected 5 nodes unchanged, got %d", len(out))
			}
			for _, n := range out {
				if n.Kind != "commit" || n.Count != 1 {
					t.Fatalf("node should stay commit/1: %+v", n)
				}
			}
		})
	}
}

func TestAggregateLinearRunsCollapsesChain(t *testing.T) {
	// A straight chain of 6 linear commits; boundaries (first and last) stay
	// as commits, the 4 interior linear commits collapse into one aggregate.
	nodes := linearChain(6)
	out := AggregateLinearRuns(nodes, 3)

	var aggregates, commits int
	var agg CommitNode
	for _, n := range out {
		switch n.Kind {
		case "aggregate":
			aggregates++
			agg = n
		case "commit":
			commits++
			if n.Count != 1 {
				t.Fatalf("commit node count must be 1: %+v", n)
			}
		default:
			t.Fatalf("unexpected kind %q", n.Kind)
		}
	}
	// First node has no parent (not linear: 1 parent required) → stays commit.
	// Last node has no child → stays commit. The 4 interior collapse.
	if aggregates != 1 {
		t.Fatalf("expected 1 aggregate, got %d (out=%+v)", aggregates, out)
	}
	if agg.Count != 4 {
		t.Fatalf("aggregate count: got %d want 4", agg.Count)
	}
	if agg.ShortHash != "+4" {
		t.Fatalf("aggregate short_hash: got %q want +4", agg.ShortHash)
	}
	if agg.Message != "4 commits collapsed" {
		t.Fatalf("aggregate message: got %q", agg.Message)
	}
	if !strings.HasPrefix(agg.Hash, "agg_1_") {
		t.Fatalf("aggregate hash prefix: got %q", agg.Hash)
	}
	// Date is the newest commit in the run (interior commits are idx 1..4).
	if !agg.Date.Equal(testBaseTime.Add(4 * time.Minute)) {
		t.Fatalf("aggregate date: got %v", agg.Date)
	}
	// The aggregate's parent is the run's external parent (node 0).
	if len(agg.Parents) != 1 || agg.Parents[0] != nodes[0].Hash {
		t.Fatalf("aggregate parents: got %v want [%s]", agg.Parents, nodes[0].Hash)
	}
	// The last node (idx 5) must now point at the aggregate, keeping the graph
	// connected (its original parent, idx 4, was collapsed).
	last := out[len(out)-1]
	if len(last.Parents) != 1 || last.Parents[0] != agg.Hash {
		t.Fatalf("last node should reparent to aggregate: got %v want [%s]", last.Parents, agg.Hash)
	}
}

func TestAggregateLinearRunsPreservesTaggedAndMerges(t *testing.T) {
	// Chain of 5 with a tag on the middle commit (idx 2) splitting the run
	// into two: idx 1 alone (not collapsible — lone run) and idx 3 alone.
	nodes := linearChain(5)
	nodes[2].Tag = "v1.0.0"
	out := AggregateLinearRuns(nodes, 2)

	// No interior run of length >= 2 exists, so nothing collapses; all stay
	// commits and the tagged node keeps its tag.
	for _, n := range out {
		if n.Kind != "commit" {
			t.Fatalf("nothing should collapse, got aggregate: %+v", n)
		}
	}
	if len(out) != 5 {
		t.Fatalf("expected 5 nodes, got %d", len(out))
	}
	// The tag survives.
	var tagged bool
	for _, n := range out {
		if n.Tag == "v1.0.0" {
			tagged = true
		}
	}
	if !tagged {
		t.Fatal("tagged node lost its tag after aggregation")
	}
}

func TestAggregateLinearRunsRelayoutAssignsCoordinates(t *testing.T) {
	nodes := linearChain(6)
	out := AggregateLinearRuns(nodes, 3)
	// After re-layout the first node sits at x_offset 0 and lanes are assigned.
	if out[0].XOffset != 0 {
		t.Fatalf("first node x_offset after relayout: got %v want 0", out[0].XOffset)
	}
	for _, n := range out {
		if n.Lane < 0 {
			t.Fatalf("node missing lane assignment: %+v", n)
		}
	}
}

func TestExtractUnifiedTopologyEmptyInput(t *testing.T) {
	nodes, err := ExtractUnifiedTopology(map[string]*git.Repository{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodes) != 0 {
		t.Fatalf("expected no nodes, got %d", len(nodes))
	}
}
