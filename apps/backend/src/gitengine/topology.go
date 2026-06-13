package gitengine

import (
	"fmt"
	"sort"
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

	// Layout pass: x_offset from age, lane from parentage. activeLanes maps
	// lane index -> prefixed hash of the latest commit occupying it. Lanes
	// are never freed (kept simple deliberately); a commit inherits its
	// primary parent's lane when that parent is still the lane tip,
	// otherwise it claims the lowest unoccupied lane.
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

	return nodes, nil
}
