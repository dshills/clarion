package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newPackCmd() *cobra.Command {
	pack := &cobra.Command{
		Use:   "pack",
		Short: "Generate documentation packages",
	}
	pack.AddCommand(newPackEnterpriseCmd())
	return pack
}

func newPackEnterpriseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "enterprise",
		Short: "Generate the full enterprise documentation package",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "pack enterprise: not yet implemented")
			return nil
		},
	}
}
