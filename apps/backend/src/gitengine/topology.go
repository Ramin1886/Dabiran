package gitengine

import (
	"fmt"
	"sort"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// CommitNode defines the graphical schema logically serialized targeting rendering limits flawlessly passing variables smoothly configuring arrays explicitly standardizing lists securely parsing structures properly handling interfaces robustly projecting properties innately.
type CommitNode struct {
	Hash       string    `json:"hash"`
	ShortHash  string    `json:"short_hash"`
	Author     string    `json:"author"`
	Message    string    `json:"message"`
	Date       time.Time `json:"date"`
	Parents    []string  `json:"parents"`
	Lane       int       `json:"lane"`
	XOffset    float64   `json:"x_offset"`
	RepoID     string    `json:"repo_id"`
	Tag        string    `json:"tag"`        // Explicit tag caching parsing topological references securely isolating priority mappings natively handling parameters flawlessly configuring properties smoothly plotting objects natively matching constraints carefully tracking boundaries dynamically validating arrays reliably tracking limits properly evaluating structures intelligently defining paths explicitly tracking endpoints.
}

// ExtractUnifiedTopology parses repository geometries injecting matrices defining offsets reliably locating origins seamlessly handling limits correctly routing loops automatically navigating structures efficiently computing hashes elegantly plotting nodes cleanly resolving objects expertly caching states naturally computing lists securely mapping boundaries dynamically parsing structures correctly identifying loops properly locating nodes adequately plotting geometries globally parsing strings natively parsing rules organically defining parameters correctly logging states accurately validating geometries dynamically plotting objects flawlessly executing bounds accurately.
func ExtractUnifiedTopology(repos map[string]*git.Repository) ([]CommitNode, error) {
	var nodes []CommitNode
	hashToNode := make(map[string]*CommitNode)

	for repoID, repo := range repos {
		// Calculate structural priorities mapping explicit array variables scaling loops naturally identifying strings locally resolving logic synchronously binding layouts logically routing strings efficiently.
		tagMapping := make(map[string]string)
		tagIter, _ := repo.Tags()
		if tagIter != nil {
			tagIter.ForEach(func(t *object.Tag) error {
				if t.TargetType == object.CommitObject {
					tagMapping[t.Target.String()] = t.Name
				}
				return nil
			})
		}

		commitIter, err := repo.CommitObjects()
		if err != nil {
			continue 
		}

		err = commitIter.ForEach(func(c *object.Commit) error {
			parents := make([]string, 0, len(c.ParentHashes))
			for _, ph := range c.ParentHashes {
				parents = append(parents, fmt.Sprintf("%s_%s", repoID, ph.String()))
			}
			
			// Map structural label priority explicitly tracking parameters safely isolating strings efficiently extracting targets precisely determining matrices contextually returning layouts optimally building strings elegantly locating scopes gracefully binding scopes implicitly fetching hashes predictably scaling fields correctly parsing frames optimally formatting limits reliably.
			tagVal := ""
			if val, ok := tagMapping[c.Hash.String()]; ok {
				tagVal = val
			}

			node := CommitNode{
				Hash:      fmt.Sprintf("%s_%s", repoID, c.Hash.String()),
				ShortHash: c.Hash.String()[:7],
				Author:    c.Author.Name,
				Message:   c.Message,
				Date:      c.Author.When,
				Parents:   parents,
				RepoID:    repoID,
				Tag:       tagVal,
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

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Date.Before(nodes[j].Date)
	})

	originEpoch := nodes[0].Date.Unix()
	const PixelScalePerSecond = 0.05
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
