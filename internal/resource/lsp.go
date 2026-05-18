package resource

import (
	"encoding/json"
	"fmt"
)

func init() {
	register(Kind{Name: "lsp", List: listLSP})
}

// lsp: lsp.json | .github/lsp.json.
func listLSP(root string) ([]Item, error) {
	data, src, ok, err := readFirstWithSource(root, "lsp.json", ".github/lsp.json")
	if err != nil || !ok {
		return nil, err
	}
	var doc struct {
		Servers map[string]any `json:"servers"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse lsp.json: %w", err)
	}
	var out []Item
	for name := range doc.Servers {
		out = append(out, Item{Slug: name, Path: src + "#" + name})
	}
	return out, nil
}
