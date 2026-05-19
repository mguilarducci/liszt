package render

// Progress drives a single Bar across a known number of phases. Step
// advances the bar percentage, updates its label, and emits a persistent
// `✓ <previous label>` line for the just-finished phase. The first Step
// has no previous label and only starts the bar.
type Progress struct {
	r       *Renderer
	bar     *Bar
	total   int
	current int
	label   string
	failed  bool
}

// NewProgress constructs a determinate Progress with total steps. The bar
// starts at 0%. Use Step to advance, Done or StepFail to terminate.
func (r *Renderer) NewProgress(total int) *Progress {
	return &Progress{
		r:     r,
		bar:   r.Bar(""),
		total: total,
	}
}

// Step advances the bar to the next phase. If a previous step is in
// flight, its label is committed as a `✓` line. Calling Step more times
// than `total` is tolerated (bar caps at 100%) but indicates a caller
// mismatch worth fixing.
func (p *Progress) Step(label string) {
	if p.failed {
		return
	}
	if p.current > 0 && p.label != "" {
		p.r.StepDone(p.label)
	}
	p.current++
	if p.total > 0 {
		pct := float64(p.current) / float64(p.total)
		if pct > 1 {
			pct = 1
		}
		p.bar.Set(pct)
	}
	p.bar.Update(label)
	p.label = label
	p.bar.repaint()
}

// Done emits a final `✓ <current label>` line, then calls Bar.Done which
// prints the `✔ msg` summary block.
func (p *Progress) Done(msg string, kv ...any) {
	if p.failed {
		return
	}
	if p.label != "" {
		p.r.StepDone(p.label)
	}
	p.bar.Set(1)
	p.bar.Done(msg, kv...)
}

// StepFail stops the bar with an `✗ <current label>: <err>` line. Any
// subsequent Step / Done call on this Progress is a no-op.
func (p *Progress) StepFail(err error) {
	if p.failed {
		return
	}
	p.failed = true
	p.bar.Stop()
	p.r.StepFail(p.label, err)
}
