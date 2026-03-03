package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newDriftCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "drift",
		Short: "Detect drift between current code and previous documentation snapshot",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "drift: not yet implemented")
			return nil
		},
	}
}
