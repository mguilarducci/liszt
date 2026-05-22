package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mguilarducci/liszt/internal/runner"
)

var runConfigPath string

var runCmd = &cobra.Command{
	Use:   "run <name>",
	Short: "Run a named task from .liszt/liszt.toml",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		cfg, err := runner.Load(runConfigPath)
		if err != nil {
			return err
		}
		target, ok := cfg.Target(args[0])
		if !ok {
			return fmt.Errorf("no [tasks.%s] target in %s", args[0], runConfigPath)
		}
		os.Exit(target.Run(args[0], os.Stdout, os.Stderr))
		return nil // coverage: unreachable, os.Exit terminates the process
	},
}

func init() {
	runCmd.Flags().StringVar(&runConfigPath, "config", ".liszt/liszt.toml", "config path")
	rootCmd.AddCommand(runCmd)
}
