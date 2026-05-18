package claudestate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// InstalledPlugin is one entry in installed_plugins.json.
type InstalledPlugin struct {
	Scope        string `json:"scope"`
	InstallPath  string `json:"installPath"`
	Version      string `json:"version"`
	InstalledAt  string `json:"installedAt"`
	LastUpdated  string `json:"lastUpdated"`
	GitCommitSha string `json:"gitCommitSha"`
}

// InstalledPlugins mirrors installed_plugins.json (schema v2).
type InstalledPlugins struct {
	Version int                          `json:"version"`
	Plugins map[string][]InstalledPlugin `json:"plugins"`
}

// LoadInstalled reads path. Missing file returns an empty v2 registry.
func LoadInstalled(path string) (*InstalledPlugins, error) {
	ip := &InstalledPlugins{Version: 2, Plugins: map[string][]InstalledPlugin{}}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ip, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, ip); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if ip.Plugins == nil {
		ip.Plugins = map[string][]InstalledPlugin{}
	}
	if ip.Version == 0 {
		ip.Version = 2
	}
	return ip, nil
}

// SaveInstalled writes ip with 2-space indent. Creates parent dirs.
func SaveInstalled(path string, ip *InstalledPlugins) error {
	data, err := json.MarshalIndent(ip, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// FindUserEntry returns the scope=user entry for key, or nil.
func (ip *InstalledPlugins) FindUserEntry(key string) *InstalledPlugin {
	for i, e := range ip.Plugins[key] {
		if e.Scope == "user" {
			return &ip.Plugins[key][i]
		}
	}
	return nil
}

// Upsert replaces the scope=user entry for key (preserving InstalledAt
// if present) or appends a new one.
func (ip *InstalledPlugins) Upsert(key string, e InstalledPlugin) {
	e.Scope = "user"
	for i, x := range ip.Plugins[key] {
		if x.Scope == "user" {
			if e.InstalledAt == "" {
				e.InstalledAt = x.InstalledAt
			}
			ip.Plugins[key][i] = e
			return
		}
	}
	ip.Plugins[key] = append(ip.Plugins[key], e)
}
