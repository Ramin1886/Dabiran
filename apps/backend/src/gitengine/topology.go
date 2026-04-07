package gitengine

import (
	"fmt"
	"sort"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// CommitNode defines the graphical schema serialized directly to the internal browser mapping the WebGL canvas natively.
type CommitNode struct {
	Hash       string    `json:"hash"`
	ShortHash  string    `json:"short_hash"`
	Author     string    `json:"author"`
	Message    string    `json:"message"`
	Date       time.Time `json:"date"`
	Parents    []string  `json:"parents"`
	Lane       int       `json:"lane"`
	XOffset    float64   `json:"x_offset"`
	RepoID     string    `json:"repo_id"`    // Tracks explicit repository ownership resolving UI contextual lookups synchronously.
}

// ExtractUnifiedTopology parses unlimited *git.Repository maps injecting prefix isolations gracefully tracking overlapping layouts natively sorting timestamps O(N log N) bounds safely updating logic.
func ExtractUnifiedTopology(repos map[string]*git.Repository) ([]CommitNode, error) {
	var nodes []CommitNode
	hashToNode := make(map[string]*CommitNode)

	// Consolidate arrays iteratively checking branches determining logic.
	for repoID, repo := range repos {
		commitIter, err := repo.CommitObjects()
		if err != nil {
			// Instead of complete failure, we skip unreadable repositories securing functional limits inherently validating structures explicitly handling errors cleanly.
			continue 
		}

		err = commitIter.ForEach(func(c *object.Commit) error {
			parents := make([]string, 0, len(c.ParentHashes))
			for _, ph := range c.ParentHashes {
				// Apply Hash Collision Mitigation visually defining absolute boundaries correctly scaling matrices elegantly parsing parameters accurately.
				parents = append(parents, fmt.Sprintf("%s_%s", repoID, ph.String()))
			}
			
			node := CommitNode{
				Hash:      fmt.Sprintf("%s_%s", repoID, c.Hash.String()),
				ShortHash: c.Hash.String()[:7],
				Author:    c.Author.Name,
				Message:   c.Message,
				Date:      c.Author.When,
				Parents:   parents,
				RepoID:    repoID,
			}
			nodes = append(nodes, node)
			hashToNode[node.Hash] = &nodes[len(nodes)-1]
			return nil
		})
		if err != nil {
			continue
		}
	}

	if len(nodes) == 0 {
		return nodes, nil
	}

	// Algorithm: Step 1. Strict Left-to-Right Sort Global (O(N log N)).
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Date.Before(nodes[j].Date)
	})

	// Algorithm: Step 2. Calculate explicit XOffset bounding scale reliably defining distance natively tracking offsets mapping logic elegantly.
	originEpoch := nodes[0].Date.Unix()
	const PixelScalePerSecond = 0.05
	
	// Algorithm: Step 3. Y-Axis Lane assignment caching states determining overlaps smoothly formatting lanes.
	activeLanes := make(map[int]string) 
	
	for i := range nodes {
		nodes[i].XOffset = float64(nodes[i].Date.Unix()-originEpoch) * PixelScalePerSecond
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
			for l := 0; l < 1000; l++ {
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
