package render

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestProgress_StepEmitsPriorCheckmark(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := newTTYRenderer(&buf)
	p := r.NewProgress(3)

	p.Step("Resolving")
	p.Step("Cloning")

	got := buf.String()
	if !strings.Contains(got, "✓ Resolving") {
		t.Errorf("second Step should print prior ✓ Resolving line: %q", got)
	}
	if !strings.Contains(got, "Cloning") {
		t.Errorf("bar should now carry Cloning label: %q", got)
	}
	p.Done("ok")
}

func TestProgress_DoneEmitsFinalCheckAndSummary(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := newTTYRenderer(&buf)
	p := r.NewProgress(2)
	p.Step("Resolving")
	p.Step("Saving")
	p.Done("Added obra/superpowers", "plugins", 1)

	got := buf.String()
	if !strings.Contains(got, "✓ Saving") {
		t.Errorf("Done should emit final ✓ for current step: %q", got)
	}
	if !strings.Contains(got, "✔ Added obra/superpowers") {
		t.Errorf("Done should emit success header: %q", got)
	}
	if !strings.Contains(got, "  plugins: 1") {
		t.Errorf("Done should emit summary kv: %q", got)
	}
}

func TestProgress_StepFailMarksFailure(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := newTTYRenderer(&buf)
	p := r.NewProgress(3)
	p.Step("Resolving")
	p.Step("Cloning")
	p.StepFail(errors.New("network down"))

	got := buf.String()
	if !strings.Contains(got, "✓ Resolving") {
		t.Errorf("prior step should still be ✓: %q", got)
	}
	if !strings.Contains(got, "✗ Cloning: network down") {
		t.Errorf("failing step should emit ✗ with err: %q", got)
	}
}

func TestProgress_NonTTYEmitsOneLinePerStep(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := New(&buf) // bytes.Buffer is non-TTY
	p := r.NewProgress(2)
	p.Step("Resolving")
	p.Step("Saving")
	p.Done("ok")

	got := buf.String()
	if !strings.Contains(got, "Resolving") || !strings.Contains(got, "Saving") {
		t.Errorf("non-TTY progress should print both step labels: %q", got)
	}
}
