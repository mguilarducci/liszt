package render

// Header prints an H1: bold + underline, no color override so the terminal
// default foreground keeps it readable on every theme.
func (r *Renderer) Header(text string) {
	r.write(styH1.Render(text) + "\n")
}

// Subheader prints an H2: pink-deep bold with a ▸ prefix.
func (r *Renderer) Subheader(text string) {
	r.write(styH2.Render("▸ "+text) + "\n")
}

// Hint prints an H3: dim italic metadata line.
func (r *Renderer) Hint(text string) {
	r.write(styH3.Render(text) + "\n")
}

func (r *Renderer) write(s string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.active != nil {
		r.eraseLine()
	}
	r.writeString(s)
	if r.active != nil {
		active := r.active
		r.mu.Unlock()
		active.repaint()
		r.mu.Lock()
	}
}
