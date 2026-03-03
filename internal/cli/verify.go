package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/clarion-dev/clarion/internal/facts"
	"github.com/clarion-dev/clarion/internal/verify"
)

func newVerifyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "verify",
		Short: "Verify generated documentation against clarion-meta.json",
		RunE: func(cmd *cobra.Command, args []string) error {
			outputDir := flagOutput

			// Pre-checks: return errors (not Fatalf) so cobra propagates them
			// through the normal error path and any deferred cleanup runs.
			mdFiles, _ := filepath.Glob(filepath.Join(outputDir, "*.md"))
			if len(mdFiles) == 0 {
				return fmt.Errorf("no documentation found in %s. Run clarion pack enterprise first", outputDir)
			}
			metaPath := filepath.Join(outputDir, "clarion-meta.json")
			if _, err := os.Stat(metaPath); err != nil {
				return fmt.Errorf("clarion-meta.json not found in %s. Run clarion pack enterprise first", outputDir)
			}

			fm, err := facts.Load(metaPath)
			if err != nil {
				return fmt.Errorf("load clarion-meta.json: %w", err)
			}

			v := verify.New(outputDir)
			reports, allPassed, err := v.VerifyAll(fm)
			if err != nil {
				return fmt.Errorf("verify: %w", err)
			}

			for _, r := range reports {
				fmt.Printf("Section %-20s  high=%.0f%% medium=%.0f%% low=%.0f%%  failed=%d\n",
					r.Section, r.HighConfidence, r.MediumConfidence, r.LowConfidence,
					len(r.FailedClaims))
			}

			if !allPassed {
				// Return ErrVerifyFailed so deferred cleanup runs normally.
				// main maps this sentinel to ExitFailure (1).
				return ErrVerifyFailed
			}
			return nil
		},
	}
}
