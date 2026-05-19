package render

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// Info prints an informational line.
func (r *Renderer) Info(msg string, kv ...any) {
	r.writeLine(styInfoBar, styInfoLbl, lblInfo, msg, kv)
}

// Warn prints a warning line.
func (r *Renderer) Warn(msg string, kv ...any) {
	r.writeLine(styWarnBar, styWarnLbl, lblWarn, msg, kv)
}

// Error prints an error line. Callers should still return the error from the
// cobra RunE — fang prints the styled terminal error separately.
func (r *Renderer) Error(msg string, kv ...any) {
	r.writeLine(styErrorBar, styErrorLbl, lblError, msg, kv)
}

// Done prints a success line.
func (r *Renderer) Done(msg string, kv ...any) {
	r.writeLine(styDoneBar, styDoneLbl, lblDone, msg, kv)
}

func (r *Renderer) writeLine(barSty, lblSty lipgloss.Style, label, msg string, kv []any) {
	line := r.formatLine(barSty, lblSty, label, msg, kv)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.active != nil {
		r.eraseLine()
	}
	r.writeString(line)
	if r.active != nil {
		// repaint takes r.mu, so drop and re-acquire (the deferred Unlock at
		// the top fires on a held lock at return).
		active := r.active
		r.mu.Unlock()
		active.repaint()
		r.mu.Lock()
	}
}

// formatLine builds:
//
//	▌ <label>  <msg>  k1=v1 k2=v2\n
//
// Continuation lines in msg are indented under the message column.
func (r *Renderer) formatLine(barSty, lblSty lipgloss.Style, label, msg string, kv []any) string {
	bar := barSty.Render("▌")
	lbl := lblSty.Render(label)
	prefix := bar + " " + lbl + "  "
	indent := strings.Repeat(" ", 1+1+len(label)+2)

	msgLines := strings.Split(msg, "\n")
	var sb strings.Builder
	for i, line := range msgLines {
		if i == 0 {
			sb.WriteString(prefix)
		} else {
			sb.WriteString(indent)
		}
		sb.WriteString(line)
		if i < len(msgLines)-1 {
			sb.WriteString("\n")
		}
	}

	if len(kv) > 0 {
		sb.WriteString("  ")
		sb.WriteString(formatKV(kv))
	}
	sb.WriteString("\n")
	return sb.String()
}

func formatKV(kv []any) string {
	parts := make([]string, 0, (len(kv)+1)/2)
	for i := 0; i < len(kv); i += 2 {
		key := fmt.Sprint(kv[i])
		var value string
		if i+1 >= len(kv) {
			value = "<missing>"
		} else {
			value = fmt.Sprint(kv[i+1])
		}
		parts = append(parts, styDim.Render(key+"="+value))
	}
	return strings.Join(parts, " ")
}
