package resource

import (
	"encoding/json"
	"fmt"
)

func init() {
	register(Kind{Name: "mcp", List: listMCP})
}

// mcp: .claude-plugin/mcp.json | .mcp.json | .github/mcp.json.
func listMCP(root string) ([]Item, error) {
	data, src, ok, err := readFirstWithSource(root, ".claude-plugin/mcp.json", ".mcp.json", ".github/mcp.json")
	if err != nil || !ok {
		return nil, err
	}
	var doc struct {
		MCPServers map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse mcp.json: %w", err)
	}
	var out []Item
	for name := range doc.MCPServers {
		out = append(out, Item{Slug: name, Path: src + "#" + name})
	}
	return out, nil
}
