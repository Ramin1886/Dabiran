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

type GitEngine struct { BaseStoragePath string }

func NewGitEngine(storagePath string) *GitEngine {
	os.MkdirAll(storagePath, 0755)
	return &GitEngine{BaseStoragePath: storagePath}
}

func (e *GitEngine) EnsureRepository(ctx context.Context, repoID int, url string, authType string, authSecret string) (*git.Repository, error) {
	repoPath := filepath.Join(e.BaseStoragePath, fmt.Sprintf("repo_%d.git", repoID))
	var authMethod git.AuthMethod
	if authType == "https" {
		authMethod = &http.BasicAuth{ Username: "x-access-token", Password: authSecret }
	} else if authType == "ssh" {
		authMethod, _ = ssh.NewPublicKeys("git", []byte(authSecret), "")
	}

	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return git.PlainCloneContext(ctx, repoPath, true, &git.CloneOptions{ URL: url, Auth: authMethod, Progress: os.Stdout })
	}
	repo, err := git.PlainOpen(repoPath)
	if err != nil { return nil, err }
	err = repo.FetchContext(ctx, &git.FetchOptions{ Auth: authMethod, Force: true, Tags: git.AllTags, RefSpecs: []config.RefSpec{"refs/*:refs/*"} })
	if err != nil && err != git.NoErrAlreadyUpToDate { return nil, err }
	return repo, nil
}
