package gitengine

import (
	"sort"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// CommitNode represents a single commit on the visualization canvas
type CommitNode struct {
	Hash       string       `json:"hash"`
	ShortHash  string       `json:"short_hash"`
	Author     string       `json:"author"`
	Message    string       `json:"message"`
	Date       time.Time    `json:"date"`
	Parents    []string     `json:"parents"` // Connections for diagonals
	BranchName string       `json:"branch_name,omitempty"` // For distinct coloring/lanes
}

// ExtractTopology reads all commits and returns a chronological slice suitable for rendering.
// This implements a lightweight topological traversal optimized for the Viewport.
func ExtractTopology(repo *git.Repository) ([]CommitNode, error) {
	// We want all commits across all branches to construct the multi-branch graph.
	commitIter, err := repo.CommitObjects()
	if err != nil {
		return nil, err
	}

	var nodes []CommitNode

	err = commitIter.ForEach(func(c *object.Commit) error {
		parents := make([]string, 0, len(c.ParentHashes))
		for _, ph := range c.ParentHashes {
			parents = append(parents, ph.String())
		}

		nodes = append(nodes, CommitNode{
			Hash:      c.Hash.String(),
			ShortHash: c.Hash.String()[:7],
			Author:    c.Author.Name,
			Message:   c.Message,
			Date:      c.Author.When,
			Parents:   parents,
		})
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort chronologically (oldest to newest for left-to-right rendering layout)
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Date.Before(nodes[j].Date)
	})

	// TODO: Branch Lane Assignment (Y-axis logic).
	// To strictly follow the "Oldest Branch Unique = Top" layout constraint:
	// We will write a deterministic branch clustering algorithm here in future iterations
	// right before pagination culling.

	return nodes, nil
}
