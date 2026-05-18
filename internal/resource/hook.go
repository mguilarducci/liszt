package resource

import (
	"encoding/json"
	"fmt"
)

func init() {
	register(Kind{Name: "hook", List: listHooks})
}

// hooks: hooks/hooks.json (Claude) or hooks.json (Copilot).
func listHooks(root string) ([]Item, error) {
	data, src, ok, err := readFirstWithSource(root, "hooks/hooks.json", "hooks.json")
	if err != nil || !ok {
		return nil, err
	}
	var doc struct {
		Hooks map[string][]any `json:"hooks"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse hooks.json: %w", err)
	}
	var out []Item
	for event, entries := range doc.Hooks {
		out = append(out, Item{
			Slug:  event,
			Path:  src + "#" + event,
			Extra: fmt.Sprintf("%d", len(entries)),
		})
	}
	return out, nil
}
