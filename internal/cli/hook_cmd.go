package cli

import (
	"os"

	"github.com/mguilarducci/liszt/internal/runner"
	"github.com/spf13/cobra"
)

var hookConfigPath string

var hookCmd = &cobra.Command{
	Use:   "hook <name> [lang...] [-- gitargs...]",
	Short: "Run a git hook from .liszt/hooks.toml",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name, langs, gitArgs := splitHookArgs(args, cmd.ArgsLenAtDash())
		cfg, err := runner.Load(hookConfigPath)
		if err != nil {
			return err
		}
		segments, err := cfg.Resolve(name, langs)
		if err != nil {
			return err
		}
		os.Exit(cfg.RunHook(name, segments, gitArgs, os.Stdout, os.Stderr))
		return nil // coverage: unreachable, os.Exit terminates the process
	},
}

// splitHookArgs partitions cobra positional args using the index of "--" (dash,
// as reported by cobra.Command.ArgsLenAtDash; -1 when absent). args[0] is the
// hook name. With no "--", every remaining arg is a lang selector. With "--" at
// index d (d >= 1), args[1:d] are lang selectors and args[d:] are git args
// forwarded to the commands. A leading "--" (d == 0, i.e. "--" before the hook
// name) is malformed; it yields no langs and forwards the remaining args, so the
// name slot is never sliced out of range.
func splitHookArgs(args []string, dash int) (name string, langs, gitArgs []string) {
	name = args[0]
	switch {
	case dash < 0:
		return name, args[1:], nil
	case dash < 1:
		return name, nil, args[1:]
	default:
		return name, args[1:dash], args[dash:]
	}
}

func init() {
	hookCmd.Flags().StringVar(&hookConfigPath, "config", ".liszt/hooks.toml", "config path")
	rootCmd.AddCommand(hookCmd)
}
