// Package render is the styled CLI output layer for the liszt binary. All
// verbs print user-facing output through this package so the look-and-feel
// stays consistent across the CLI surface. See the design spec at
// docs/superpowers/specs/2026-05-18-cli-color-render-design.md.
package render

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

var (
	cPinkDeep   color.Color = lipgloss.Color("#fe7ab2")
	cPinkBright color.Color = lipgloss.Color("#ffaff3")
	cInfo       color.Color = lipgloss.Color("#9ce7ff")
	cDone       color.Color = lipgloss.Color("#aadd8b")
	cWarn       color.Color = lipgloss.Color("#ffc501")
	cError      color.Color = lipgloss.Color("#f44747")
	cDim        color.Color = lipgloss.Color("#c4c4c4")
)

var (
	styH1   = lipgloss.NewStyle().Bold(true).Underline(true)
	styH2   = lipgloss.NewStyle().Foreground(cPinkDeep).Bold(true)
	styH3   = lipgloss.NewStyle().Foreground(cDim).Italic(true)
	styDim  = lipgloss.NewStyle().Foreground(cDim)
	styPink = lipgloss.NewStyle().Foreground(cPinkBright).Bold(true)

	styDoneBar  = lipgloss.NewStyle().Foreground(cDone)
	styWarnBar  = lipgloss.NewStyle().Foreground(cWarn)
	styErrorBar = lipgloss.NewStyle().Foreground(cError)
	styStepBar  = lipgloss.NewStyle().Foreground(cInfo)

	styDoneLbl  = lipgloss.NewStyle().Foreground(cDone).Bold(true)
	styWarnLbl  = lipgloss.NewStyle().Foreground(cWarn).Bold(true)
	styErrorLbl = lipgloss.NewStyle().Foreground(cError).Bold(true)
	styStepLbl  = lipgloss.NewStyle().Foreground(cInfo).Bold(true)
)

const (
	lblDone = "done "
	lblWarn = "warn "
	lblStep = "step "
)

// Palette exposes the Gleam palette colors used by the render package. Other
// packages (notably the CLI's fang integration) use these to keep fang's
// styled help/version/error output aligned with the in-app message colors.
var Palette = struct {
	PinkDeep   color.Color
	PinkBright color.Color
	Info       color.Color
	Done       color.Color
	Warn       color.Color
	Error      color.Color
	Dim        color.Color
}{
	PinkDeep:   cPinkDeep,
	PinkBright: cPinkBright,
	Info:       cInfo,
	Done:       cDone,
	Warn:       cWarn,
	Error:      cError,
	Dim:        cDim,
}
