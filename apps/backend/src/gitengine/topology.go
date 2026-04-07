package gitengine

import (
	"sort"
	"time"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type CommitNode struct { Hash string; ShortHash string; Author string; Message string; Date time.Time; Parents []string; BranchName string; }

func ExtractTopology(repo *git.Repository) ([]CommitNode, error) {
	commitIter, err := repo.CommitObjects()
	if err != nil { return nil, err }
	var nodes []CommitNode
	commitIter.ForEach(func(c *object.Commit) error {
		parents := make([]string, 0, len(c.ParentHashes))
		for _, ph := range c.ParentHashes { parents = append(parents, ph.String()) }
		nodes = append(nodes, CommitNode{ Hash: c.Hash.String(), ShortHash: c.Hash.String()[:7], Author: c.Author.Name, Message: c.Message, Date: c.Author.When, Parents: parents })
		return nil
	})
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Date.Before(nodes[j].Date) })
	return nodes, nil
}
