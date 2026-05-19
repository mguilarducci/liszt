package render

import (
	"io"
	"math/rand/v2"
	"os"
	"sync"

	"github.com/charmbracelet/colorprofile"
)

// Renderer is the styled output engine. Construct via New or use the
// package-level Default through the delegating top-level functions in
// default.go.
type Renderer struct {
	w       io.Writer
	profile colorprofile.Profile
	isTTY   bool
	mu      sync.Mutex
	rng     *rand.Rand
	active  anim
	verbose bool
}

// anim is the surface a Renderer needs to ask the currently-active animation
// (e.g. a progress bar) to redraw itself after a print clears its line.
type anim interface {
	repaint()
}

// writeString writes s through a colorprofile.Writer keyed on the Renderer's
// profile. The Writer strips/downgrades color escape sequences on
// NoTTY/ASCII/ANSI/ANSI256 profiles and passes TrueColor through untouched.
// Cursor-control sequences (\r, \x1b[K) that must survive a NoColor profile
// go through eraseLine instead — the profile writer's ansi.Strip path would
// otherwise consume them along with the color codes.
func (r *Renderer) writeString(s string) {
	cw := &colorprofile.Writer{Forward: r.w, Profile: r.profile}
	_, _ = cw.Write([]byte(s))
}

// eraseLine emits the cursor-control sequence that returns the cursor to
// column 0 and clears the current line. Bypasses the colorprofile.Writer so
// the sequence survives on NoColor profiles. Reserved for the bar redraw
// path.
func (r *Renderer) eraseLine() {
	_, _ = r.w.Write([]byte("\r\x1b[K"))
}

// Option mutates a Renderer at construction time.
type Option func(*Renderer)

// WithProfile overrides the auto-detected color profile.
func WithProfile(p colorprofile.Profile) Option {
	return func(r *Renderer) { r.profile = p }
}

// WithNoColor forces no-color output regardless of environment.
func WithNoColor() Option {
	return func(r *Renderer) { r.profile = colorprofile.NoTTY }
}

// WithTTY overrides the auto-detected TTY status. Test-only.
func WithTTY(isTTY bool) Option {
	return func(r *Renderer) { r.isTTY = isTTY }
}

// WithRand swaps the random source used by the progress bar's flicker.
// Test-only — production callers should leave this unset.
func WithRand(rng *rand.Rand) Option {
	return func(r *Renderer) { r.rng = rng }
}

// SetVerbose toggles emission of Detail lines. Default false.
func (r *Renderer) SetVerbose(on bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.verbose = on
}

// New constructs a Renderer writing to w. Auto-detection of color profile
// and TTY status runs before options apply, so options always win.
func New(w io.Writer, opts ...Option) *Renderer {
	profile, isTTY := detectProfile(w, os.Environ())
	r := &Renderer{
		w:       w,
		profile: profile,
		isTTY:   isTTY,
		// G404 false positive: this rand drives a visual flicker, not crypto.
		rng: rand.New(rand.NewPCG(uint64(os.Getpid()), 0xDEADBEEF)), //nolint:gosec
	}
	for _, o := range opts {
		o(r)
	}
	return r
}
