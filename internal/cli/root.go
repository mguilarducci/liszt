package cli

import (
	"context"
	"fmt"
	"image/color"
	"os"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"

	"github.com/mguilarducci/liszt/internal/intro"
	"github.com/mguilarducci/liszt/internal/render"
	"github.com/mguilarducci/liszt/internal/version"
)

var rootCmd = &cobra.Command{
	Use:           "liszt",
	Short:         "liszt — agent-agnostic plugin package manager",
	Version:       version.Full(),
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, _ []string) error {
		// Bare `liszt` invocation: play the intro animation (no-op on
		// non-TTY) and then print the same help fang would render.
		_ = intro.Play(os.Stderr, true)
		return cmd.Help()
	},
}

var noColor bool

func init() {
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable color output")
	rootCmd.PersistentPreRun = func(_ *cobra.Command, _ []string) {
		if noColor {
			// Setenv only fails on platforms without env support; failure
			// here means colors stay on, which is harmless.
			_ = os.Setenv("NO_COLOR", "1")
		}
	}
}

// Execute runs the root command through charmbracelet/fang. fang styles
// --help, --version, and error output using the Gleam palette so its output
// matches the in-app render package. Callers should pass a
// context.Background() unless they need cancellation semantics.
func Execute(ctx context.Context) error {
	if err := fang.Execute(ctx, rootCmd, fang.WithColorSchemeFunc(gleamColorScheme)); err != nil {
		return fmt.Errorf("execute: %w", err)
	}
	return nil
}

// gleamColorScheme maps the Gleam palette onto fang's ColorScheme so help,
// version, and error output share the same look-and-feel as render.Info,
// render.Bar, etc. The function signature matches fang.ColorSchemeFunc and
// is invoked with a lipgloss.LightDarkFunc that resolves to the terminal's
// preferred variant; we ignore the light/dark argument because the Gleam
// palette is dark-tuned (see spec §15 on light-mode being out of scope).
func gleamColorScheme(_ lipgloss.LightDarkFunc) fang.ColorScheme {
	return fang.ColorScheme{
		Base:           color.Color(nil),
		Title:          render.Palette.PinkDeep,
		Description:    render.Palette.Dim,
		Codeblock:      color.Color(nil),
		Program:        render.Palette.PinkDeep,
		DimmedArgument: render.Palette.Dim,
		Comment:        render.Palette.Dim,
		Flag:           render.Palette.Info,
		FlagDefault:    render.Palette.Dim,
		Command:        render.Palette.Info,
		QuotedString:   render.Palette.Warn,
		Argument:       render.Palette.Done,
		Help:           render.Palette.Dim,
		Dash:           render.Palette.Dim,
		ErrorHeader:    [2]color.Color{render.Palette.PinkBright, render.Palette.Error},
		ErrorDetails:   render.Palette.Error,
	}
}
