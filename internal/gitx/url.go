package gitx

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

// ParseGitHubURL extracts owner and repo from a github.com URL.
func ParseGitHubURL(raw string) (string, string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", "", fmt.Errorf("invalid url: %w", err)
	}
	if u.Host != "github.com" {
		return "", "", fmt.Errorf("only github.com URLs supported, got %q", u.Host)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("URL must include owner/repo")
	}
	return parts[0], strings.TrimSuffix(parts[1], ".git"), nil
}

// RepoPath returns cacheDir/owner/repo.
func RepoPath(cacheDir, owner, repo string) string {
	return filepath.Join(cacheDir, owner, repo)
}
