package cli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mguilarducci/liszt/internal/repos"
)

func TestRepoAdd_AlreadyAddedReturnsSentinel(t *testing.T) {
	dir := t.TempDir()
	reposPath := filepath.Join(dir, "repos.toml")
	cache := filepath.Join(dir, "cache")

	if err := repos.Save(reposPath, &repos.Config{Repos: []repos.Entry{
		{Name: "obra/superpowers", URL: "https://github.com/obra/superpowers", SHA: "deadbeef"},
	}}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	err := RepoAdd(Paths{Repos: reposPath, Cache: cache}, "https://github.com/obra/superpowers")
	if !errors.Is(err, ErrAlreadyAdded) {
		t.Fatalf("expected ErrAlreadyAdded, got %v", err)
	}

	if _, statErr := os.Stat(filepath.Join(cache, "obra", "superpowers")); !os.IsNotExist(statErr) {
		t.Errorf("preflight should refuse before clone, but cache dir exists: %v", statErr)
	}
}
