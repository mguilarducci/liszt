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

func TestInfo_PlainOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	newTestRenderer(&buf).Info("hello")
	got := buf.String()
	if !strings.HasPrefix(got, "▌ info   hello") {
		t.Errorf("Info output missing prefix: %q", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("Info output missing newline: %q", got)
	}
}

func TestInfo_WithKV(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	newTestRenderer(&buf).Info("hello", "key", "value", "n", 42)
	got := buf.String()
	if !strings.Contains(got, "key=value") {
		t.Errorf("missing key=value: %q", got)
	}
	if !strings.Contains(got, "n=42") {
		t.Errorf("missing n=42: %q", got)
	}
}

func TestInfo_OddKV(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	newTestRenderer(&buf).Info("hello", "lonely")
	if !strings.Contains(buf.String(), "lonely=<missing>") {
		t.Errorf("odd kv should render <missing>: %q", buf.String())
	}
}

func TestEachLevelLabel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		fn    func(*Renderer, string, ...any)
		label string
	}{
		{"info", (*Renderer).Info, "info "},
		{"warn", (*Renderer).Warn, "! "},
		{"done", (*Renderer).Done, "✔ "},
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

func TestMultilineMessageIndents(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	newTestRenderer(&buf).Info("line1\nline2")
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines; got %d: %q", len(lines), buf.String())
	}
	if !strings.HasPrefix(lines[1], "        ") {
		t.Errorf("continuation line not indented: %q", lines[1])
	}
}
