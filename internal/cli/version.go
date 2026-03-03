package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newVersionCmd(version, commit, built string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("clarion %s\n", version)
			fmt.Printf("commit: %s\n", commit)
			fmt.Printf("built:  %s\n", built)
			return nil
		},
	}
}
