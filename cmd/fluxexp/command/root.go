// Package command holds the cobra command tree for the fluxexp CLI.
package command

import "github.com/spf13/cobra"

// NewRootCmd builds the root `fluxexp` command. The version is resolved by the
// caller and surfaced through cobra's own `--version` flag, which it adds by
// itself as soon as Version is non-empty — there is no flag to declare here.
func NewRootCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "fluxexp",
		Short:         "Traverse FluxCD resource graphs to their ultimate resources",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newTraverseCmd())
	cmd.AddCommand(newUICmd())
	return cmd
}
