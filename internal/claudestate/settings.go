package claudestate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// EnableSettingPlugin reads ~/.claude/settings.json as an untyped map,
// sets enabledPlugins[key] = true, writes back with 2-space indent.
// Creates the file with a minimal payload if missing.
func EnableSettingPlugin(path, key string) error {
	root := map[string]any{}
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := json.Unmarshal(data, &root); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
	case os.IsNotExist(err):
		// fall through with empty root
	default:
		return err
	}

	enabled, _ := root["enabledPlugins"].(map[string]any)
	if enabled == nil {
		enabled = map[string]any{}
	}
	enabled[key] = true
	root["enabledPlugins"] = enabled

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, out, 0644)
}
