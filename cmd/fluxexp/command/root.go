// Package command holds the cobra command tree for the fluxexp CLI.
package command

import "github.com/spf13/cobra"

// NewRootCmd builds the root `fluxexp` command.
func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "fluxexp",
		Short:         "Traverse FluxCD resource graphs to their ultimate resources",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newTraverseCmd())
	return cmd
}
