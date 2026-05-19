package resource

// Item is a discovered resource within a plugin tree.
type Item struct {
	Slug  string
	Path  string
	Extra string
}

// Lister returns the items of a given kind under a plugin root.
type Lister func(pluginRoot string) ([]Item, error)

// Kind describes how to discover items of a resource kind.
type Kind struct {
	Name string
	List Lister
}

var (
	orderedKinds []Kind
	byName       = map[string]Kind{}
)

func register(k Kind) {
	if _, exists := byName[k.Name]; exists {
		return
	}
	orderedKinds = append(orderedKinds, k)
	byName[k.Name] = k
}

// All returns every registered kind in a stable order.
func All() []Kind { return orderedKinds }

// Kinds returns the registered kind names in stable order.
func Kinds() []string {
	out := make([]string, 0, len(orderedKinds))
	for _, k := range orderedKinds {
		out = append(out, k.Name)
	}
	return out
}

// Get returns the kind with the given name.
func Get(name string) (Kind, bool) {
	k, ok := byName[name]
	return k, ok
}
