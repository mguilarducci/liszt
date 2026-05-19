package render

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// Warn prints `! <msg>` in the warn color. Kv payload is dropped (callers
// pair Warn with Detail for technical context).
func (r *Renderer) Warn(msg string, _ ...any) {
	r.writeGlyphLine("! "+msg+"\n", &styWarnLbl)
}

// Done prints `✔ <msg>` followed by an indented `key: value` summary block.
func (r *Renderer) Done(msg string, kv ...any) {
	r.writeSummaryBlock(styDoneLbl, "✔ "+msg, kv)
}

// Fail prints `✖ <msg>` followed by an indented `key: value` summary block.
// Callers should still return the underlying error from RunE so fang prints
// its own styled terminal error.
func (r *Renderer) Fail(msg string, kv ...any) {
	r.writeSummaryBlock(styErrorLbl, "✖ "+msg, kv)
}

func (r *Renderer) writeSummaryBlock(headerSty lipgloss.Style, header string, kv []any) {
	styledHeader := headerSty.Render(header)
	body := formatSummary(kv)
	r.writeGlyphLine(styledHeader+"\n"+body, nil)
}

// formatSummary renders kv pairs as aligned `  key: value\n` lines.
func formatSummary(kv []any) string {
	if len(kv) == 0 {
		return ""
	}
	keys := make([]string, 0, (len(kv)+1)/2)
	vals := make([]string, 0, (len(kv)+1)/2)
	width := 0
	for i := 0; i < len(kv); i += 2 {
		k := fmt.Sprint(kv[i])
		var v string
		if i+1 >= len(kv) {
			v = "<missing>"
		} else {
			v = fmt.Sprint(kv[i+1])
		}
		if len(k) > width {
			width = len(k)
		}
		keys = append(keys, k)
		vals = append(vals, v)
	}
	var sb strings.Builder
	for i, k := range keys {
		pad := strings.Repeat(" ", width-len(k))
		sb.WriteString("  ")
		sb.WriteString(k)
		sb.WriteString(":")
		sb.WriteString(pad)
		sb.WriteString(" ")
		sb.WriteString(vals[i])
		sb.WriteString("\n")
	}
	return sb.String()
}

// formatLine builds the bar-prefixed `▌ <label>  <msg>  k=v\n` line that
// the Bar's repaint path emits.
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
