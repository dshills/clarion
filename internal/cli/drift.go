package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/clarion-dev/clarion/internal/drift"
	"github.com/clarion-dev/clarion/internal/facts"
	"github.com/clarion-dev/clarion/internal/scanner"
)

func newDriftCmd() *cobra.Command {
	var driftThreshold float64

	cmd := &cobra.Command{
		Use:   "drift",
		Short: "Detect drift between current code and previous documentation snapshot",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Pre-check: clarion-meta.json must exist.
			metaPath := filepath.Join(flagOutput, "clarion-meta.json")
			if _, err := os.Stat(metaPath); err != nil {
				return fmt.Errorf("clarion-meta.json not found. Run clarion pack enterprise to generate an initial snapshot.")
			}

			// 1. Load previous clarion-meta.json.
			previous, err := facts.Load(metaPath)
			if err != nil {
				return fmt.Errorf("load clarion-meta.json: %w", err)
			}

			// 2. Validate --drift-threshold is in [0.0, 1.0].
			if driftThreshold < 0.0 || driftThreshold > 1.0 {
				return fmt.Errorf("drift-threshold must be in [0.0, 1.0]")
			}

			// 3. Re-scan the repo (use directory of flagSpec as repo root).
			repoRoot := filepath.Dir(flagSpec)
			s := scanner.New()
			current, err := s.Scan(repoRoot)
			if err != nil {
				return fmt.Errorf("scan: %w", err)
			}

			// 4. Compare previous and current fact models.
			report := drift.Compare(previous, current)

			// 5. Set threshold and ExceededThreshold.
			report.Threshold = driftThreshold
			report.ExceededThreshold = report.DriftFraction > driftThreshold

			// 6. Write drift-report.json to flagOutput.
			reportJSON, err := json.MarshalIndent(report, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal drift report: %w", err)
			}
			jsonPath := filepath.Join(flagOutput, "drift-report.json")
			if err := os.WriteFile(jsonPath, reportJSON, 0o644); err != nil {
				return fmt.Errorf("write drift-report.json: %w", err)
			}

			// 7. Write drift-report.md to flagOutput.
			mdPath := filepath.Join(flagOutput, "drift-report.md")
			if err := os.WriteFile(mdPath, []byte(report.Markdown()), 0o644); err != nil {
				return fmt.Errorf("write drift-report.md: %w", err)
			}

			// 8. Print summary to stdout.
			fmt.Printf("Drift fraction: %.4f (threshold: %.4f)\n", report.DriftFraction, driftThreshold)
			fmt.Printf("Added: %d  Removed: %d  Modified: %d\n",
				len(report.Added), len(report.Removed), len(report.Modified))

			// 9. Return sentinel if threshold exceeded.
			if report.ExceededThreshold {
				return ErrDriftExceeded
			}
			return nil
		},
	}

	cmd.Flags().Float64Var(&driftThreshold, "drift-threshold", 0.0, "Drift fraction threshold [0.0, 1.0]; exit 1 if exceeded")

	return cmd
}
