// Package render exposes the Gleam color palette used to style the liszt
// CLI's help, version, and error output via charmbracelet/fang.
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

// Palette exposes the Gleam palette colors. The CLI's fang integration uses
// these to style help/version/error output.
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
