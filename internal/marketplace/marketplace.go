package marketplace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Marketplace mirrors marketplace.json.
type Marketplace struct {
	Name     string   `json:"name"`
	Plugins  []Plugin `json:"plugins"`
	Metadata struct {
		PluginRoot string `json:"pluginRoot"`
	} `json:"metadata"`
}

// Plugin is one entry in Marketplace.Plugins.
type Plugin struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version,omitempty"`
	Source      any    `json:"source,omitempty"`
}

// Read reads marketplace.json from one of the canonical locations and reports
// which flavor ("claude" | "copilot") the location implies.
func Read(repoRoot string) (*Marketplace, string, error) {
	data, src, ok, err := readFirstWithSource(repoRoot,
		".claude-plugin/marketplace.json",
		".github/plugin/marketplace.json",
	)
	if err != nil {
		return nil, "", fmt.Errorf("read marketplace.json: %w", err)
	}
	if !ok {
		return nil, "", fmt.Errorf("marketplace.json not found (tried .claude-plugin/, .github/plugin/)")
	}
	var mp Marketplace
	if err := json.Unmarshal(data, &mp); err != nil {
		return nil, "", fmt.Errorf("parse marketplace.json: %w", err)
	}
	flavor := "claude"
	if strings.HasPrefix(src, ".github/plugin/") {
		flavor = "copilot"
	}
	return &mp, flavor, nil
}

// ResolvePluginPath combines the marketplace's PluginRoot with a plugin's source path.
func (m *Marketplace) ResolvePluginPath(p Plugin) string {
	src := pluginSourcePath(p.Source)
	base := strings.TrimPrefix(m.Metadata.PluginRoot, "./")
	switch {
	case base == "" || base == ".":
		return src
	case src == "":
		return base
	default:
		return filepath.Join(base, src)
	}
}

func pluginSourcePath(src any) string {
	switch v := src.(type) {
	case string:
		p := strings.TrimPrefix(v, "./")
		if p == "" || p == "." {
			return ""
		}
		return p
	case map[string]any:
		if p, ok := v["path"].(string); ok {
			return strings.TrimPrefix(p, "./")
		}
	}
	return ""
}

func readFirstWithSource(root string, paths ...string) ([]byte, string, bool, error) {
	for _, p := range paths {
		data, err := os.ReadFile(filepath.Join(root, p))
		if err == nil {
			return data, p, true, nil
		}
		if !os.IsNotExist(err) {
			return nil, "", false, err
		}
	}
	return nil, "", false, nil
}
