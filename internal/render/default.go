package render

import (
	"os"
	"sync"
)

// Default is the package-level Renderer used by the top-level delegating
// functions. It is constructed lazily on first use so tests and CLI startup
// can mutate env (NO_COLOR, CLICOLOR_FORCE, ...) before any render call
// triggers construction.
var (
	defaultOnce sync.Once
	Default     *Renderer
)

func ensureDefault() *Renderer {
	defaultOnce.Do(func() {
		Default = New(os.Stderr)
	})
	return Default
}

// Info delegates to Default.Info.
func Info(msg string, kv ...any) { ensureDefault().Info(msg, kv...) }

// Warn delegates to Default.Warn.
func Warn(msg string, kv ...any) { ensureDefault().Warn(msg, kv...) }

// Fail delegates to Default.Fail.
func Fail(msg string, kv ...any) { ensureDefault().Fail(msg, kv...) }

// Done delegates to Default.Done.
func Done(msg string, kv ...any) { ensureDefault().Done(msg, kv...) }

// Header delegates to Default.Header.
func Header(text string) { ensureDefault().Header(text) }

// Subheader delegates to Default.Subheader.
func Subheader(text string) { ensureDefault().Subheader(text) }

// Hint delegates to Default.Hint.
func Hint(text string) { ensureDefault().Hint(text) }

// NewBar delegates to Default.Bar. Named NewBar (not Bar) to avoid the
// package-level identifier colliding with the *Bar type.
func NewBar(label string) *Bar { return ensureDefault().Bar(label) }

// Detail delegates to Default.Detail.
func Detail(msg string, kv ...any) { ensureDefault().Detail(msg, kv...) }

// SetVerbose toggles Detail emission on the package-level Default renderer.
func SetVerbose(on bool) { ensureDefault().SetVerbose(on) }

// Step delegates to Default.Step.
func Step(msg string) { ensureDefault().Step(msg) }

// StepDone delegates to Default.StepDone.
func StepDone(msg string) { ensureDefault().StepDone(msg) }

// StepFail delegates to Default.StepFail.
func StepFail(msg string, err error) { ensureDefault().StepFail(msg, err) }

// NewProgress delegates to Default.NewProgress.
func NewProgress(total int) *Progress { return ensureDefault().NewProgress(total) }
