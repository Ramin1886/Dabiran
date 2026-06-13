// Package gitengine syncs remote repositories into a local bare-repo cache
// and extracts the unified commit topology served by the API.
package gitengine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

// GitEngine manages the on-disk cache of bare repositories under
// BaseStoragePath (one "repo_<id>.git" directory per tracked repository).
type GitEngine struct{ BaseStoragePath string }

// NewGitEngine creates (if needed) the storage directory and returns an
// engine rooted at storagePath.
func NewGitEngine(storagePath string) *GitEngine {
	os.MkdirAll(storagePath, 0755)
	return &GitEngine{BaseStoragePath: storagePath}
}

// EnsureRepository returns an up-to-date bare clone of url cached as
// repo_<repoID>.git. On first use it clones; afterwards it opens the cache
// and fetches all refs. authType selects "https" (PAT basic auth) or "ssh"
// (private key in authSecret); any other value fetches anonymously.
func (e *GitEngine) EnsureRepository(ctx context.Context, repoID int, url string, authType string, authSecret string) (*git.Repository, error) {
	repoPath := filepath.Join(e.BaseStoragePath, fmt.Sprintf("repo_%d.git", repoID))
	var authMethod transport.AuthMethod
	if authType == "https" {
		authMethod = &http.BasicAuth{Username: "x-access-token", Password: authSecret}
	} else if authType == "ssh" {
		authMethod, _ = ssh.NewPublicKeys("git", []byte(authSecret), "")
	}

	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return git.PlainCloneContext(ctx, repoPath, true, &git.CloneOptions{URL: url, Auth: authMethod, Progress: os.Stdout})
	}
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, err
	}
	err = repo.FetchContext(ctx, &git.FetchOptions{Auth: authMethod, Force: true, Tags: git.AllTags, RefSpecs: []config.RefSpec{"refs/*:refs/*"}})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return nil, err
	}
	return repo, nil
}
