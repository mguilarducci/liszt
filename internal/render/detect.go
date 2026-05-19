package render

import (
	"io"
	"os"

	"github.com/charmbracelet/colorprofile"
	"github.com/mattn/go-isatty"
)

// detectProfile reads the writer and environment to determine the color
// profile and whether the writer is attached to a terminal. The environ
// slice mirrors os.Environ() so tests can drive it without mutating real env.
//
// colorprofile.Detect already honors NO_COLOR / CLICOLOR / CLICOLOR_FORCE /
// TERM=dumb / COLORTERM. We layer on a strict isTTY check because the bar
// goroutine spawns only when isTTY is true.
func detectProfile(w io.Writer, environ []string) (colorprofile.Profile, bool) {
	profile := colorprofile.Detect(w, environ)
	isTTY := writerIsTTY(w)
	return profile, isTTY
}

// writerIsTTY reports whether w writes to a terminal. Returns false for any
// non-*os.File writer (e.g. bytes.Buffer in tests).
func writerIsTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isatty.IsTerminal(f.Fd())
}
