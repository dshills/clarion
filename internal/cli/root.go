package cli

import (
	"os"

	"github.com/spf13/cobra"
)

// Global flag values shared across all commands.
var (
	flagSpec    string
	flagPlan    string
	flagOutput  string
	flagJSON    bool
	flagVerbose bool
)

// New returns the root cobra command for clarion.
func New(version, commit, built string) *cobra.Command {
	root := &cobra.Command{
		Use:           "clarion",
		Short:         "Make systems legible",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Skip flag validation for version and help.
			if cmd.Name() == "version" {
				return nil
			}

			// Enforce --json and --verbose mutual exclusion.
			if flagJSON && flagVerbose {
				return exitError("--json and --verbose are mutually exclusive")
			}

			SetJSONMode(flagJSON)

			// Validate --spec exists and is readable (except for version).
			if _, err := os.Stat(flagSpec); err != nil {
				Fatalf("spec file not found or unreadable: %s", flagSpec)
			}

			return nil
		},
	}

	root.PersistentFlags().StringVar(&flagSpec, "spec", "./SPEC.md", "Path to SPEC.md")
	root.PersistentFlags().StringVar(&flagPlan, "plan", "./PLAN.md", "Path to PLAN.md (optional)")
	root.PersistentFlags().StringVar(&flagOutput, "output", "./docs", "Output directory for generated files")
	root.PersistentFlags().BoolVar(&flagJSON, "json", false, "Emit structured JSON to stdout")
	root.PersistentFlags().BoolVar(&flagVerbose, "verbose", false, "Print step-by-step processing details to stderr")

	root.AddCommand(
		newPackCmd(),
		newGenCmd(),
		newDriftCmd(),
		newVerifyCmd(),
		newVersionCmd(version, commit, built),
	)

	return root
}

// exitError returns a simple error that causes cobra to print usage.
type cliError struct{ msg string }

func (e *cliError) Error() string { return e.msg }

func exitError(msg string) error { return &cliError{msg: msg} }
