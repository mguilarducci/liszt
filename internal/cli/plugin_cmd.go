package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	pluginCmd = &cobra.Command{
		Use:   "plugin",
		Short: "Manage plugins",
	}

	pluginListCmd = &cobra.Command{
		Use:   "list",
		Short: "List plugins across all repos",
		RunE: func(_ *cobra.Command, _ []string) error {
			return PluginList(defaultPaths())
		},
	}

	pluginInstallFlavor string

	pluginInstallCmd = &cobra.Command{
		Use:   "install <slug>",
		Short: "Install a plugin",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if err := validateFlavor(pluginInstallFlavor); err != nil {
				return err
			}
			return PluginInstall(defaultPaths(), args[0], pluginInstallFlavor)
		},
	}
)

func init() {
	pluginInstallCmd.Flags().StringVar(&pluginInstallFlavor, "flavor", "", "claude|copilot")
	_ = pluginInstallCmd.MarkFlagRequired("flavor")

	pluginCmd.AddCommand(pluginListCmd)
	pluginCmd.AddCommand(pluginInstallCmd)
	rootCmd.AddCommand(pluginCmd)
}

func validateFlavor(flavor string) error {
	if flavor != "claude" && flavor != "copilot" {
		return fmt.Errorf("--flavor must be 'claude' or 'copilot', got %q", flavor)
	}
	return nil
}
