package gitengine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestNewGitEngineCreatesStorageDir(t *testing.T) {
	base := filepath.Join(t.TempDir(), "nested", "repos")
	engine := NewGitEngine(base)
	if engine.BaseStoragePath != base {
		t.Fatalf("BaseStoragePath: got %s want %s", engine.BaseStoragePath, base)
	}
	if info, err := os.Stat(base); err != nil || !info.IsDir() {
		t.Fatalf("storage dir not created: %v", err)
	}
}

func TestEnsureRepositoryClonesThenFetches(t *testing.T) {
	srcDir, hashes := buildSourceRepo(t)
	engine := NewGitEngine(t.TempDir())
	ctx := context.Background()

	// First call clones a bare cache.
	repo, err := engine.EnsureRepository(ctx, 7, srcDir, "", "")
	if err != nil {
		t.Fatalf("initial clone: %v", err)
	}
	cachePath := filepath.Join(engine.BaseStoragePath, fmt.Sprintf("repo_%d.git", 7))
	if _, err := os.Stat(cachePath); err != nil {
		t.Fatalf("bare cache missing at %s: %v", cachePath, err)
	}
	if _, err := repo.CommitObject(hashes.c2); err != nil {
		t.Fatalf("cloned repo missing commit c2: %v", err)
	}

	// Second call opens the cache and fetches (up-to-date is not an error).
	if _, err := engine.EnsureRepository(ctx, 7, srcDir, "", ""); err != nil {
		t.Fatalf("refresh fetch: %v", err)
	}
}
