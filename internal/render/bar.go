package render

import (
	"fmt"
	"math"
	"strings"
	"sync/atomic"
	"time"
)

// Bar is a single-line progress bar. Filled cells re-roll a random note from
// {♬, ♪, ♩, ♫} on every tick. Construct via Renderer.Bar.
//
// Callers must invoke exactly one of Stop, Done, or Fail.
type Bar struct {
	r             *Renderer
	label         atomic.Value  // string
	pct           atomic.Uint64 // float64 via math.Float64bits
	indeterminate atomic.Bool
	width         int
	notes         []string
	tick          time.Duration
	stop          chan struct{}
	done          chan struct{}
	stopped       atomic.Bool
	loopActive    atomic.Bool
}

// Bar constructs a new progress bar with the given initial label.
func (r *Renderer) Bar(label string) *Bar {
	b := &Bar{
		r:     r,
		width: 24,
		notes: []string{"♬", "♪", "♩", "♫"},
		tick:  100 * time.Millisecond,
		stop:  make(chan struct{}),
		done:  make(chan struct{}),
	}
	b.label.Store(label)
	b.pct.Store(math.Float64bits(0))
	if !r.isTTY {
		// Non-TTY: emit a single info line on creation. Stop/Done/Fail emit
		// the final line. No animation loop, no cursor control.
		line := r.formatLine(styInfoBar, styInfoLbl, lblInfo, label, nil)
		r.mu.Lock()
		r.writeString(line)
		r.mu.Unlock()
		close(b.done)
		return b
	}
	r.mu.Lock()
	r.active = b
	r.mu.Unlock()
	b.loopActive.Store(true)
	go b.loop()
	return b
}

func (b *Bar) loop() {
	defer close(b.done)
	t := time.NewTicker(b.tick)
	defer t.Stop()
	for {
		select {
		case <-b.stop:
			return
		case <-t.C:
			b.repaint()
		}
	}
}

// repaint renders one frame of the bar in its current mode.
func (b *Bar) repaint() {
	pct := math.Float64frombits(b.pct.Load())
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	label, _ := b.label.Load().(string)
	cells := b.renderCells(pct)
	bar := styInfoBar.Render("▌")
	lbl := styInfoLbl.Render(lblInfo)
	var pctStr string
	if b.indeterminate.Load() {
		pctStr = styDim.Render("····")
	} else {
		pctStr = styDim.Render(fmt.Sprintf("%3d%%", int(pct*100)))
	}
	b.r.mu.Lock()
	defer b.r.mu.Unlock()
	b.r.eraseLine()
	b.r.writeString(fmt.Sprintf("%s %s  %s  %s  %s", bar, lbl, cells, pctStr, label))
}

// renderCells assumes pct is already clamped to [0,1] by the caller.
func (b *Bar) renderCells(pct float64) string {
	filled := int(pct * float64(b.width))
	if b.indeterminate.Load() {
		// Indeterminate: every cell uses the filled-flicker treatment so the
		// bar looks alive without claiming a percentage.
		filled = b.width
	}
	var sb strings.Builder
	for i := range b.width {
		if i < filled {
			sb.WriteString(styPink.Render(b.notes[b.r.rng.IntN(len(b.notes))]))
		} else {
			sb.WriteString(styDim.Render("·"))
		}
	}
	return sb.String()
}

// Set updates the bar's percentage. Values are clamped to [0,1]. In
// indeterminate mode the stored value is preserved but not displayed.
func (b *Bar) Set(pct float64) { b.pct.Store(math.Float64bits(pct)) }

// SetIndeterminate toggles indeterminate mode. In indeterminate mode all
// cells animate with the flicker pattern and the percentage column shows
// dim dots instead of a number. Used for opaque long operations (e.g.
// `git clone`) where progress cannot be measured.
func (b *Bar) SetIndeterminate(on bool) { b.indeterminate.Store(on) }

// Update changes the label. No-op on non-TTY (matches the non-TTY one-shot
// line emitted at construction).
func (b *Bar) Update(label string) { b.label.Store(label) }

// Stop ends the animation and clears the bar line. No-op on non-TTY.
func (b *Bar) Stop() {
	if !b.r.isTTY {
		return
	}
	if !b.stopped.CompareAndSwap(false, true) {
		return
	}
	if b.loopActive.CompareAndSwap(true, false) {
		close(b.stop)
		<-b.done
	}
	b.r.mu.Lock()
	b.r.active = nil
	b.r.eraseLine()
	b.r.mu.Unlock()
}

// Done stops the bar and prints a Done line.
func (b *Bar) Done(msg string, kv ...any) {
	b.Stop()
	b.r.Done(msg, kv...)
}

// Fail stops the bar and prints an Error line.
func (b *Bar) Fail(msg string, kv ...any) {
	b.Stop()
	b.r.Error(msg, kv...)
}
