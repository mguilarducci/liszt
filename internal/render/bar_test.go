package render

import (
	"bytes"
	"math/rand/v2"
	"strings"
	"testing"
)

func newTTYRenderer(buf *bytes.Buffer) *Renderer {
	return New(buf, WithTTY(true), WithRand(rand.New(rand.NewPCG(1, 2))))
}

func TestBar_SetClampsToZeroOne(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	b := newTTYRenderer(&buf).Bar("label")
	b.Set(-0.5)
	b.repaint()
	if !strings.Contains(buf.String(), "  0%") {
		t.Errorf("Set(-0.5) should clamp to 0%%: %q", buf.String())
	}

	buf.Reset()
	b.Set(2.0)
	b.repaint()
	if !strings.Contains(buf.String(), "100%") {
		t.Errorf("Set(2.0) should clamp to 100%%: %q", buf.String())
	}
	b.Stop()
}

func TestBar_DoneEmitsSuccessLine(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	b := newTTYRenderer(&buf).Bar("label")
	b.Done("installed", "slug", "x")
	got := buf.String()
	if !strings.Contains(got, "done ") {
		t.Errorf("Done did not emit done line: %q", got)
	}
	if !strings.Contains(got, "slug=x") {
		t.Errorf("Done did not include kv: %q", got)
	}
}

func TestBar_FailEmitsErrorLine(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	b := newTTYRenderer(&buf).Bar("label")
	b.Fail("boom", "err", "network")
	if !strings.Contains(buf.String(), "error") {
		t.Errorf("Fail did not emit error line: %q", buf.String())
	}
}

func TestBar_UpdateChangesLabel(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	b := newTTYRenderer(&buf).Bar("first")
	b.Update("second")
	b.repaint()
	if !strings.Contains(buf.String(), "second") {
		t.Errorf("Update did not change label: %q", buf.String())
	}
	b.Stop()
}

func TestBar_IndeterminateShowsDots(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	b := newTTYRenderer(&buf).Bar("cloning")
	b.SetIndeterminate(true)
	b.Set(0.5)
	b.repaint()
	got := buf.String()
	if !strings.Contains(got, "····") {
		t.Errorf("indeterminate did not render dots: %q", got)
	}
	if strings.Contains(got, " 50%") {
		t.Errorf("indeterminate should hide percentage: %q", got)
	}
	b.Stop()
}

func TestBar_IndeterminateBackToDeterminate(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	b := newTTYRenderer(&buf).Bar("step")
	b.SetIndeterminate(true)
	b.repaint()
	buf.Reset()
	b.SetIndeterminate(false)
	b.Set(0.5)
	b.repaint()
	if !strings.Contains(buf.String(), " 50%") {
		t.Errorf("returning to determinate did not show percentage: %q", buf.String())
	}
	b.Stop()
}

func TestBar_NonTTYSingleLineOnConstruction(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := New(&buf) // bytes.Buffer is non-TTY
	r.Bar("hello")
	got := buf.String()
	if !strings.Contains(got, "step") {
		t.Errorf("non-TTY bar should emit step line: %q", got)
	}
	if !strings.Contains(got, "hello") {
		t.Errorf("non-TTY bar missing label text: %q", got)
	}
}

func TestBar_NonTTYDoneEmitsDoneLine(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := New(&buf)
	b := r.Bar("hello")
	b.Done("finished")
	got := buf.String()
	if !strings.Contains(got, "done") {
		t.Errorf("non-TTY Done should emit done line: %q", got)
	}
	if !strings.Contains(got, "finished") {
		t.Errorf("non-TTY Done missing message: %q", got)
	}
}

func TestBar_StopIdempotent(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	b := newTTYRenderer(&buf).Bar("x")
	b.Stop()
	b.Stop() // must not panic, must not double-close
}

func TestBar_MidPrintInterruptRepaints(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := newTTYRenderer(&buf)
	b := r.Bar("running")
	b.Set(0.5)
	r.Step("interrupting message")
	got := buf.String()
	if !strings.Contains(got, "interrupting message") {
		t.Errorf("Step line missing: %q", got)
	}
	if !strings.Contains(got, "running") {
		t.Errorf("bar did not repaint after Step: %q", got)
	}
	b.Stop()
}
