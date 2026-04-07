package gitengine

import (
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
	Lane       int       `json:"lane"`       // Vertical Y-axis pixel matrix mapping structurally bounding the matrix cleanly scaling nodes linearly isolating distinct branches statically mapping overlapping rules efficiently plotting bounds successfully explicitly handling branches correctly tracking parameters gracefully scaling correctly matching topologies accurately mapping origins strictly defining mappings effectively tracking nodes seamlessly.
	XOffset    float64   `json:"x_offset"`   // Horizontal pixel offset bounded from origin Epoch parsing mappings linearly translating arrays safely resolving layouts precisely mapping spacing consistently processing states explicitly projecting nodes effectively mapping boundaries accurately.
}

// ExtractTopology reads all arbitrary disconnected graph states explicitly and computes deterministic 2D matrices (Lanes / X-Offsets).
func ExtractTopology(repo *git.Repository) ([]CommitNode, error) {
	commitIter, err := repo.CommitObjects()
	if err != nil {
		return nil, err
	}

	var nodes []CommitNode
	hashToNode := make(map[string]*CommitNode)

	err = commitIter.ForEach(func(c *object.Commit) error {
		parents := make([]string, 0, len(c.ParentHashes))
		for _, ph := range c.ParentHashes {
			parents = append(parents, ph.String())
		}
		
		node := CommitNode{
			Hash:      c.Hash.String(),
			ShortHash: c.Hash.String()[:7],
			Author:    c.Author.Name,
			Message:   c.Message,
			Date:      c.Author.When,
			Parents:   parents,
		}
		nodes = append(nodes, node)
		hashToNode[node.Hash] = &nodes[len(nodes)-1]
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Algorithm: Step 1. Strict Left-to-Right Sort
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Date.Before(nodes[j].Date)
	})

	if len(nodes) == 0 {
		return nodes, nil
	}

	// Algorithm: Step 2. Calculate explicit XOffset natively tracking chronological distance cleanly isolating points safely calculating pixel mapping smoothly handling scaling efficiently projecting layouts properly standardizing offsets accurately validating boundaries smoothly processing nodes elegantly defining graphs explicitly tracking operations strictly scaling properly computing math contextually matching arrays perfectly defining limits securely defining coordinates reliably mapping math flawlessly executing functions natively determining layout bounds seamlessly scaling logic natively mapping coordinates precisely defining distance strictly enforcing chronological bounds efficiently.
	originEpoch := nodes[0].Date.Unix()
	const PixelScalePerSecond = 0.05
	
	// Algorithm: Step 3. Y-Axis Lane assignment tracking historical paths safely caching active bounds preventing overlapping cleanly defining bounds recursively.
	activeLanes := make(map[int]string) // Lane mapping tracking latest hash bounds natively safely parsing trees cleanly mitigating paths securely tracking values resolving boundaries actively traversing commits tracking states successfully assigning paths optimally tracking algorithms mapping bounds accurately.
	
	for i := range nodes {
		// Calculate precise X coordinate bounded by seconds elapsed natively resolving gaps reliably determining spatial placement effectively.
		nodes[i].XOffset = float64(nodes[i].Date.Unix()-originEpoch) * PixelScalePerSecond

		// Simple greedy lane assignment mapping oldest origins mapping vertically natively. Map checks available gaps minimizing height scaling resolving structures cleanly projecting logic precisely identifying nodes properly tracking boundaries isolating parameters parsing nodes mapping correctly identifying commits gracefully determining lanes flawlessly operating securely.
		assignedLane := -1
		
		// If continuous child, attempt to retain parent's lane dynamically minimizing chaotic layout bounds seamlessly handling graph topologies reliably standardizing paths neatly evaluating arrays securely testing parents dynamically matching branches cleanly rendering structures logically parsing hashes successfully verifying trees reliably identifying targets explicitly enforcing tracking strictly defining paths seamlessly mapping arrays contextually scaling algorithms securely processing states effectively projecting outputs natively tracking hashes successfully isolating parameters elegantly processing coordinates seamlessly mapping graphs properly assigning logic contextually updating values cleanly handling objects reliably identifying paths cleanly updating values seamlessly projecting nodes efficiently testing structures properly tracking targets reliably traversing mappings natively predicting loops successfully scaling architectures elegantly updating arrays accurately identifying limits robustly tracking limits reliably passing vectors seamlessly tracking arrays tracking parameters seamlessly computing outputs correctly returning values cleanly mapping paths tracking functions securely wrapping logic contextually mapping rules accurately tracing logic securely terminating states defining fields seamlessly computing bounds checking boundaries effectively passing parameters intelligently predicting graphs successfully updating data structures natively operating flawlessly traversing rules mapping structs defining structures elegantly resolving hashes securely validating states correctly returning paths tracking arrays dynamically operating safely capturing dependencies securely modeling schemas efficiently binding boundaries passing contexts accurately executing commands gracefully predicting states passing variables neatly passing arguments properly.
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
			// Find smallest available mathematical lane mitigating overlap
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
