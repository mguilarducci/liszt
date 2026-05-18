package claudestate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// MarketplaceSource is the "source" object in known_marketplaces.json.
type MarketplaceSource struct {
	Source string `json:"source"` // "github"
	Repo   string `json:"repo"`   // "owner/name"
}

// KnownMarketplace is one entry in known_marketplaces.json.
type KnownMarketplace struct {
	Source          MarketplaceSource `json:"source"`
	InstallLocation string            `json:"installLocation"`
	LastUpdated     string            `json:"lastUpdated"`
}

// KnownMarketplaces maps marketplace name to its registration.
type KnownMarketplaces map[string]KnownMarketplace

// LoadKnown reads path. Missing file returns an empty map.
func LoadKnown(path string) (KnownMarketplaces, error) {
	km := KnownMarketplaces{}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return km, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, &km); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return km, nil
}

// SaveKnown writes km with 2-space indent. Creates parent dirs.
func SaveKnown(path string, km KnownMarketplaces) error {
	data, err := json.MarshalIndent(km, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// UpsertMarketplace inserts name if absent. If present with a different
// Source.Repo, returns a conflict error. LastUpdated is always refreshed.
func (km KnownMarketplaces) UpsertMarketplace(name string, src MarketplaceSource, installLocation, now string) error {
	if cur, ok := km[name]; ok {
		if cur.Source.Repo != src.Repo {
			return fmt.Errorf("marketplace %q already registered with different source %q; resolve manually", name, cur.Source.Repo)
		}
		cur.LastUpdated = now
		cur.InstallLocation = installLocation
		km[name] = cur
		return nil
	}
	km[name] = KnownMarketplace{Source: src, InstallLocation: installLocation, LastUpdated: now}
	return nil
}
