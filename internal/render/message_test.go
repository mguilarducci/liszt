package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

func newTestRenderer(buf *bytes.Buffer) *Renderer {
	return New(buf, WithProfile(colorprofile.NoTTY))
}

func TestEachLevelLabel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		fn    func(*Renderer, string, ...any)
		label string
	}{
		{"warn", (*Renderer).Warn, "! "},
		{"done", (*Renderer).Done, "✔ "},
		{"fail", (*Renderer).Fail, "✖ "},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var buf bytes.Buffer
			c.fn(newTestRenderer(&buf), "msg")
			if !strings.Contains(buf.String(), c.label) {
				t.Errorf("level %s missing label %q: %q", c.name, c.label, buf.String())
			}
		})
	}
}

func TestFail_GlyphAndSummaryBlock(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	newTestRenderer(&buf).Fail("repos save failed", "path", "/tmp/x", "err", "permission denied")
	got := buf.String()
	if !strings.HasPrefix(got, "✖ repos save failed\n") {
		t.Errorf("Fail missing glyph header: %q", got)
	}
	if !strings.Contains(got, "  path: /tmp/x\n") {
		t.Errorf("Fail missing indented kv path: %q", got)
	}
	if !strings.Contains(got, "  err:  permission denied\n") {
		t.Errorf("Fail missing indented kv err: %q", got)
	}
}

func TestFail_NoKVOmitsSummary(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	newTestRenderer(&buf).Fail("boom")
	got := buf.String()
	if got != "✖ boom\n" {
		t.Errorf("Fail with no kv mismatch: %q", got)
	}
}

func TestDone_SummaryBlock(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	newTestRenderer(&buf).Done("Added obra/superpowers", "marketplace", "superpowers-dev", "plugins", 1)
	got := buf.String()
	if !strings.HasPrefix(got, "✔ Added obra/superpowers\n") {
		t.Errorf("Done missing glyph header: %q", got)
	}
	if !strings.Contains(got, "  marketplace: superpowers-dev\n") {
		t.Errorf("Done missing marketplace kv: %q", got)
	}
	if !strings.Contains(got, "  plugins:     1\n") {
		t.Errorf("Done missing plugins kv (aligned): %q", got)
	}
}

func TestWarn_GlyphOnly(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	newTestRenderer(&buf).Warn("marketplace.json missing")
	got := buf.String()
	if got != "! marketplace.json missing\n" {
		t.Errorf("Warn output mismatch: %q", got)
	}
}

func TestFormatKVOddPairsMarksMissing(t *testing.T) {
	t.Parallel()

	got := formatKV([]any{"lonely"})
	if !strings.Contains(got, "lonely=<missing>") {
		t.Errorf("odd kv should render <missing>: %q", got)
	}
}

func TestFormatSummaryOddPairsMarksMissing(t *testing.T) {
	t.Parallel()

	got := formatSummary([]any{"orphan"})
	if !strings.Contains(got, "orphan: <missing>") {
		t.Errorf("odd summary kv should render <missing>: %q", got)
	}
}
