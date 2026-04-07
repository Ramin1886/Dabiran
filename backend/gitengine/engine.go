package gitengine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

type GitEngine struct {
	BaseStoragePath string
}

// NewGitEngine creates a new git engine bound to a specific local cache directory for bare repositories.
func NewGitEngine(storagePath string) *GitEngine {
	if err := os.MkdirAll(storagePath, 0755); err != nil {
		panic(fmt.Sprintf("failed to create storage path: %v", err))
	}
	return &GitEngine{BaseStoragePath: storagePath}
}

// EnsureRepository clones or fetches the latest state of a bare repository.
func (e *GitEngine) EnsureRepository(ctx context.Context, repoID int, url string, authType string, authSecret string) (*git.Repository, error) {
	repoPath := filepath.Join(e.BaseStoragePath, fmt.Sprintf("repo_%d.git", repoID))

	var authMethod git.AuthMethod
	if authType == "https" {
		authMethod = &http.BasicAuth{
			Username: "x-access-token", // Common for generic PATs
			Password: authSecret,
		}
	} else if authType == "ssh" {
		publicKeys, err := ssh.NewPublicKeys("git", []byte(authSecret), "")
		if err != nil {
			return nil, fmt.Errorf("invalid ssh key: %w", err)
		}
		authMethod = publicKeys
	}

	// Check if repository already exists locally
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		// Bare clone it
		repo, err := git.PlainCloneContext(ctx, repoPath, true, &git.CloneOptions{
			URL:      url,
			Auth:     authMethod,
			Progress: os.Stdout,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to clone repository: %w", err)
		}
		return repo, nil
	}

	// Open existing bare repository and fetch updates
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open existing local repository: %w", err)
	}

	err = repo.FetchContext(ctx, &git.FetchOptions{
		Auth:     authMethod,
		Force:    true,
		Tags:     git.AllTags,
		RefSpecs: []config.RefSpec{"refs/*:refs/*"}, 
	})

	if err != nil && err != git.NoErrAlreadyUpToDate {
		return nil, fmt.Errorf("failed to fetch updates: %w", err)
	}

	return repo, nil
}
