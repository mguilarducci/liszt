package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mguilarducci/liszt/internal/render"
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

func TestRepoAdd_AlreadyAddedEmitsFailAndHint(t *testing.T) {
	var buf bytes.Buffer
	prev := render.Default
	render.Default = render.New(&buf)
	t.Cleanup(func() { render.Default = prev })

	dir := t.TempDir()
	reposPath := filepath.Join(dir, "repos.toml")
	if err := repos.Save(reposPath, &repos.Config{Repos: []repos.Entry{
		{Name: "obra/superpowers", URL: "https://github.com/obra/superpowers", SHA: "1"},
	}}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	_ = RepoAdd(Paths{Repos: reposPath, Cache: dir + "/cache"},
		"https://github.com/obra/superpowers")

	got := buf.String()
	if !strings.Contains(got, "✓ Resolved obra/superpowers") {
		t.Errorf("missing resolve step: %q", got)
	}
	if !strings.Contains(got, "✖ obra/superpowers already added") {
		t.Errorf("missing fail line: %q", got)
	}
	if !strings.Contains(got, "→ Run `liszt repo update obra/superpowers`") {
		t.Errorf("missing hint: %q", got)
	}
}
