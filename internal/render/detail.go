package render

import (
	"strings"
)

// Detail prints a dim diagnostic line of the form `· msg k=v ...`. It is
// suppressed unless SetVerbose(true) has been called on the receiver.
//
// Detail is the home for technical payload (paths, SHAs, URLs) that used
// to ride along on Info lines in earlier versions of the CLI.
func (r *Renderer) Detail(msg string, kv ...any) {
	r.mu.Lock()
	if !r.verbose {
		r.mu.Unlock()
		return
	}
	r.mu.Unlock()

	var sb strings.Builder
	sb.WriteString("· ")
	sb.WriteString(msg)
	if len(kv) > 0 {
		sb.WriteString(" ")
		sb.WriteString(formatKV(kv))
	}
	line := styDim.Render(sb.String()) + "\n"

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.active != nil {
		r.eraseLine()
	}
	r.writeString(line)
	if r.active != nil {
		active := r.active
		r.mu.Unlock()
		active.repaint()
		r.mu.Lock()
	}
}
