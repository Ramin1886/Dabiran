package gitengine

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// CommitNode is the wire schema for one commit on the unified canvas. The
// JSON tags are the contract with packages/shared-types/src/index.ts
// (CommitNode) — keep them snake_case and in sync.
type CommitNode struct {
	Hash      string    `json:"hash"`
	ShortHash string    `json:"short_hash"`
	Author    string    `json:"author"`
	Message   string    `json:"message"`
	Date      time.Time `json:"date"`
	Parents   []string  `json:"parents"`
	Lane      int       `json:"lane"`
	XOffset   float64   `json:"x_offset"`
	RepoID    string    `json:"repo_id"`
	// Tag is the tag name when the commit is tagged, "" otherwise. The UI
	// label priority is Tag > short_hash.
	Tag string `json:"tag"`
	// Kind is "commit" for a real commit (the default) or "aggregate" for a
	// synthetic node collapsing a linear run produced by AggregateLinearRuns.
	Kind string `json:"kind"`
	// Count is the number of underlying commits represented by this node: 1
	// for a normal commit, the run length for an aggregate.
	Count int `json:"count"`
}

// pixelScalePerSecond converts commit age (seconds since the oldest commit)
// into the canvas x_offset.
const pixelScalePerSecond = 0.05

// resolveTags maps commit SHA -> tag name for every tag ref in repo.
// Lightweight tags point directly at a commit; annotated tags point at a tag
// object whose target commit is resolved here. Unresolvable refs are skipped.
func resolveTags(repo *git.Repository) map[string]string {
	tags := make(map[string]string)
	iter, err := repo.Tags()
	if err != nil {
		return tags
	}
	iter.ForEach(func(ref *plumbing.Reference) error {
		name := ref.Name().Short()
		if tagObj, err := repo.TagObject(ref.Hash()); err == nil {
			// Annotated tag: resolve the tag object's target commit.
			if commit, err := tagObj.Commit(); err == nil {
				tags[commit.Hash.String()] = name
			}
			return nil
		}
		// Lightweight tag: the ref hash is the commit itself.
		tags[ref.Hash().String()] = name
		return nil
	})
	return tags
}

// ExtractUnifiedTopology flattens the commits of every repository in repos
// (keyed by repo ID) into a single chronologically sorted node list with
// layout metadata. Hashes and parent hashes are prefixed "<RepoID>_<SHA>" so
// nodes from different repositories never collide. x_offset grows with
// seconds elapsed since the oldest commit; lanes follow the primary parent
// where possible, otherwise the lowest free lane is claimed.
func ExtractUnifiedTopology(repos map[string]*git.Repository) ([]CommitNode, error) {
	var nodes []CommitNode

	for repoID, repo := range repos {
		tagMapping := resolveTags(repo)

		commitIter, err := repo.CommitObjects()
		if err != nil {
			continue
		}
		commitIter.ForEach(func(c *object.Commit) error {
			parents := make([]string, 0, len(c.ParentHashes))
			for _, ph := range c.ParentHashes {
				parents = append(parents, fmt.Sprintf("%s_%s", repoID, ph.String()))
			}
			nodes = append(nodes, CommitNode{
				Hash:      fmt.Sprintf("%s_%s", repoID, c.Hash.String()),
				ShortHash: c.Hash.String()[:7],
				Author:    c.Author.Name,
				Message:   c.Message,
				Date:      c.Author.When,
				Parents:   parents,
				RepoID:    repoID,
				Tag:       tagMapping[c.Hash.String()],
				Kind:      "commit",
				Count:     1,
			})
			return nil
		})
	}

	if len(nodes) == 0 {
		return nodes, nil
	}

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Date.Before(nodes[j].Date)
	})

	layoutNodes(nodes)
	return nodes, nil
}

// layoutNodes assigns x_offset (from commit age) and lane (from parentage) to
// every node in chronologically sorted nodes, in place. activeLanes maps lane
// index -> prefixed hash of the latest commit occupying it. Lanes are never
// freed (kept simple deliberately); a commit inherits its primary parent's
// lane when that parent is still the lane tip, otherwise it claims the lowest
// unoccupied lane. It is safe to re-run after AggregateLinearRuns so the
// coordinates stay consistent with the collapsed node set.
func layoutNodes(nodes []CommitNode) {
	if len(nodes) == 0 {
		return
	}
	originEpoch := nodes[0].Date.Unix()
	activeLanes := make(map[int]string)

	for i := range nodes {
		nodes[i].XOffset = float64(nodes[i].Date.Unix()-originEpoch) * pixelScalePerSecond

		assignedLane := -1
		if len(nodes[i].Parents) > 0 {
			primaryParent := nodes[i].Parents[0]
			for lane, hash := range activeLanes {
				if hash == primaryParent {
					assignedLane = lane
					break
				}
			}
		}
		if assignedLane == -1 {
			for l := 0; ; l++ {
				if _, occupied := activeLanes[l]; !occupied {
					assignedLane = l
					break
				}
			}
		}

		nodes[i].Lane = assignedLane
		activeLanes[assignedLane] = nodes[i].Hash
	}
}

// AggregateLinearRuns collapses maximal runs of linear commits into single
// "aggregate" nodes when len(nodes) exceeds maxNodes (and maxNodes > 0),
// returning a new, re-laid-out slice. When maxNodes <= 0 or the node count is
// already within budget, nodes is returned unchanged.
//
// A commit is "linear" when it has exactly one parent and exactly one child
// (within the supplied node set), and carries no tag. A maximal run of such
// commits — together bounded by an external parent and an external child — is
// replaced by one node:
//
//	kind="aggregate", count=<run length>
//	hash="agg_<repoID>_<firstSHA>_<lastSHA>" (oldest..newest in the run)
//	short_hash="+<count>", message="<count> commits collapsed"
//	date=newest commit's date, parents=the run's external parent(s)
//
// Splits (multiple children), merges (multiple parents), tagged commits, and
// the commits at run boundaries remain kind="commit". After collapsing, the
// lane/x_offset layout is recomputed so coordinates stay consistent. Hashes
// referencing collapsed commits (a child whose parent fell inside a run) are
// rewritten to point at the replacement aggregate node so the graph stays
// connected.
func AggregateLinearRuns(nodes []CommitNode, maxNodes int) []CommitNode {
	if maxNodes <= 0 || len(nodes) <= maxNodes {
		return nodes
	}

	// childCount[hash] = number of nodes in the set that list hash as a parent.
	childCount := make(map[string]int, len(nodes))
	for i := range nodes {
		for _, p := range nodes[i].Parents {
			childCount[p]++
		}
	}

	// linear reports whether node n is an interior, collapsible commit:
	// exactly one parent, exactly one child, untagged, and a plain commit.
	linear := func(n CommitNode) bool {
		return n.Kind == "commit" && n.Tag == "" && len(n.Parents) == 1 && childCount[n.Hash] == 1
	}

	out := make([]CommitNode, 0, len(nodes))
	// remap rewrites any parent hash that pointed at a now-collapsed commit to
	// the aggregate node that replaced it.
	remap := make(map[string]string)

	for i := 0; i < len(nodes); {
		if !linear(nodes[i]) {
			out = append(out, nodes[i])
			i++
			continue
		}
		// Begin a maximal run of linear commits starting at i. nodes are
		// chronologically sorted (oldest first), so nodes[i] is the oldest in
		// the run and the run's external parent is nodes[i].Parents[0].
		j := i
		for j < len(nodes) && linear(nodes[j]) {
			j++
		}
		run := nodes[i:j]
		if len(run) == 1 {
			// A lone linear commit is not worth collapsing; keep it.
			out = append(out, nodes[i])
			i = j
			continue
		}
		oldest := run[0]
		newest := run[len(run)-1]
		oldestSHA := strings.TrimPrefix(oldest.Hash, oldest.RepoID+"_")
		newestSHA := strings.TrimPrefix(newest.Hash, newest.RepoID+"_")
		aggHash := fmt.Sprintf("agg_%s_%s_%s", oldest.RepoID, oldestSHA, newestSHA)

		agg := CommitNode{
			Hash:      aggHash,
			ShortHash: fmt.Sprintf("+%d", len(run)),
			Author:    newest.Author,
			Message:   fmt.Sprintf("%d commits collapsed", len(run)),
			Date:      newest.Date,
			Parents:   append([]string(nil), oldest.Parents...),
			RepoID:    oldest.RepoID,
			Kind:      "aggregate",
			Count:     len(run),
		}
		// Any node that listed the newest collapsed commit as its parent must
		// now point at the aggregate instead.
		remap[newest.Hash] = aggHash
		out = append(out, agg)
		i = j
	}

	if len(remap) > 0 {
		for i := range out {
			for k, p := range out[i].Parents {
				if replacement, ok := remap[p]; ok {
					out[i].Parents[k] = replacement
				}
			}
		}
	}

	layoutNodes(out)
	return out
}
