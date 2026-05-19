package render

import (
	"os"
	"sync"
)

// Default is the package-level Renderer used by the top-level delegating
// functions. It is constructed lazily on first use so tests and CLI startup
// can mutate env (NO_COLOR, CLICOLOR_FORCE, ...) before any render call
// triggers construction.
var (
	defaultOnce sync.Once
	Default     *Renderer
)

func ensureDefault() *Renderer {
	defaultOnce.Do(func() {
		Default = New(os.Stderr)
	})
	return Default
}
