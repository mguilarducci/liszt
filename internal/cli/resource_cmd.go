package cli

import (
	"github.com/spf13/cobra"

	"github.com/mguilarducci/liszt/internal/resource"
)

func init() {
	for _, kind := range resource.Kinds() {
		registerResourceCmd(kind)
	}
}

// registerResourceCmd builds the parent subcommand and its list/install
// children for a single resource kind, then attaches them to rootCmd. The
// closure captures kind so each child handler routes to the correct kind in
// the resource package.
func registerResourceCmd(kind string) {
	parent := &cobra.Command{
		Use:   kind,
		Short: "Manage " + kind + "s",
	}

	var listPlugin string
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List " + kind + "s",
		RunE: func(_ *cobra.Command, _ []string) error {
			return ResourceList(defaultPaths(), kind, listPlugin)
		},
	}
	listCmd.Flags().StringVar(&listPlugin, "plugin", "", "filter by plugin name")

	var installFlavor string
	installCmd := &cobra.Command{
		Use:   "install <slug>",
		Short: "Install a " + kind,
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if err := validateFlavor(installFlavor); err != nil {
				return err
			}
			return ResourceInstall(defaultPaths(), kind, args[0], installFlavor)
		},
	}
	installCmd.Flags().StringVar(&installFlavor, "flavor", "", "claude|copilot")
	_ = installCmd.MarkFlagRequired("flavor")

	parent.AddCommand(listCmd)
	parent.AddCommand(installCmd)
	rootCmd.AddCommand(parent)
}
