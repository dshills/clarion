package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newGenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "gen [section]",
		Short: "Generate a single documentation section",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "gen %s: not yet implemented\n", args[0])
			return nil
		},
	}
}
