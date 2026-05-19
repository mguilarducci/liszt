package render

import (
	"strings"

	"charm.land/lipgloss/v2"
)

type lipglossStyle = lipgloss.Style

// Step emits `⠇ <msg>` as a persistent line above any active bar. The glyph
// is static; the bar (when present) carries the live animation.
func (r *Renderer) Step(msg string) {
	r.writeGlyphLine("⠇ "+msg+"\n", nil)
}

// StepDone emits `✓ <msg>` dim. Used to mark a finished progress phase.
func (r *Renderer) StepDone(msg string) {
	r.writeGlyphLine("✓ "+msg+"\n", &styDim)
}

// StepFail emits `✗ <msg>: <err>` in the error color. A nil err drops the
// trailing `: <err>` portion so a bare failed-step line is still legible.
func (r *Renderer) StepFail(msg string, err error) {
	var b strings.Builder
	b.WriteString("✗ ")
	b.WriteString(msg)
	if err != nil {
		b.WriteString(": ")
		b.WriteString(err.Error())
	}
	b.WriteString("\n")
	r.writeGlyphLine(b.String(), &styErrorLbl)
}

// writeGlyphLine writes a glyph-prefixed verb line through the same
// mid-print erase/repaint dance writeLine uses. If sty is non-nil the
// raw string (minus trailing newline, to avoid lipgloss block padding) is
// wrapped in that style before write.
func (r *Renderer) writeGlyphLine(raw string, sty *lipglossStyle) {
	out := raw
	if sty != nil {
		trimmed := strings.TrimSuffix(raw, "\n")
		out = sty.Render(trimmed) + "\n"
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.active != nil {
		r.eraseLine()
	}
	r.writeString(out)
	if r.active != nil {
		active := r.active
		r.mu.Unlock()
		active.repaint()
		r.mu.Lock()
	}
}
