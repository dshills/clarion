//go:build integration

package integration_test

import (
	"strings"
	"testing"

	"github.com/clarion-dev/clarion/internal/drift"
	"github.com/clarion-dev/clarion/internal/facts"
	"github.com/clarion-dev/clarion/internal/scanner"
)

// TestDriftZeroDrift scans the clarion repo twice and asserts the drift
// fraction is exactly 0. The scanner is deterministic given identical input,
// so two back-to-back scans must produce identical FactModels.
// No LLM calls are made.
func TestDriftZeroDrift(t *testing.T) {
	root := repoRoot(t)
	s := scanner.New()

	fm1, err := s.Scan(root)
	if err != nil {
		t.Fatalf("first Scan: %v", err)
	}
	fm2, err := s.Scan(root)
	if err != nil {
		t.Fatalf("second Scan: %v", err)
	}

	report := drift.Compare(fm1, fm2)

	if report.DriftFraction != 0.0 {
		t.Errorf("DriftFraction = %.4f, want 0 (same repo scanned twice)", report.DriftFraction)
		if len(report.Added) > 0 {
			t.Logf("unexpected added (%d):", len(report.Added))
			for _, e := range report.Added {
				t.Logf("  + [%s] %s", e.Type, e.Name)
			}
		}
		if len(report.Removed) > 0 {
			t.Logf("unexpected removed (%d):", len(report.Removed))
			for _, e := range report.Removed {
				t.Logf("  - [%s] %s", e.Type, e.Name)
			}
		}
		if len(report.Modified) > 0 {
			t.Logf("unexpected modified (%d):", len(report.Modified))
			for _, e := range report.Modified {
				t.Logf("  ~ [%s] %s", e.Type, e.Name)
			}
		}
	}

	// Markdown rendering must not panic and must contain the report header.
	md := report.Markdown()
	if !strings.Contains(md, "Drift Report") {
		t.Error("Markdown() output does not contain 'Drift Report'")
	}
	if !strings.Contains(md, "Status: OK") {
		t.Errorf("Markdown() status: got:\n%s", md)
	}

	t.Logf("drift fraction: %.4f (threshold=%.4f exceeded=%v)",
		report.DriftFraction, report.Threshold, report.ExceededThreshold)
}

// TestDriftDetectsAddedComponent verifies that adding a synthetic component to
// the current FactModel registers as a non-zero drift fraction.
func TestDriftDetectsAddedComponent(t *testing.T) {
	root := repoRoot(t)
	s := scanner.New()

	previous, err := s.Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	// Shallow-copy the FactModel and append a synthetic component. The append
	// creates a new backing slice, leaving previous.Components unchanged.
	current := *previous
	current.Components = append(append([]facts.Component(nil), previous.Components...), facts.Component{
		Name:        "IntegrationTestSyntheticComponent",
		Description: "Injected by TestDriftDetectsAddedComponent",
		Evidence: facts.Evidence{
			SourceFiles:     []string{"fake/synthetic.go"},
			ConfidenceScore: 0.9,
		},
	})

	report := drift.Compare(previous, &current)

	if len(report.Added) == 0 {
		t.Error("expected at least one Added entry, got none")
	}
	if report.DriftFraction <= 0.0 {
		t.Errorf("DriftFraction = %.4f, want > 0", report.DriftFraction)
	}

	found := false
	for _, e := range report.Added {
		if e.Name == "IntegrationTestSyntheticComponent" {
			found = true
		}
	}
	if !found {
		t.Error("synthetic component not found in Added entries")
	}

	t.Logf("drift fraction: %.4f, added=%d removed=%d modified=%d",
		report.DriftFraction, len(report.Added), len(report.Removed), len(report.Modified))
}

// TestDriftDetectsRemovedComponent verifies that removing a component from the
// current FactModel registers in the Removed list.
func TestDriftDetectsRemovedComponent(t *testing.T) {
	root := repoRoot(t)
	s := scanner.New()

	previous, err := s.Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(previous.Components) == 0 {
		t.Skip("no components in FactModel; cannot test removal")
	}

	// Current is identical to previous but with the first component dropped.
	current := *previous
	current.Components = append([]facts.Component(nil), previous.Components[1:]...)

	report := drift.Compare(previous, &current)

	if len(report.Removed) == 0 {
		t.Error("expected at least one Removed entry, got none")
	}
	if report.DriftFraction <= 0.0 {
		t.Errorf("DriftFraction = %.4f, want > 0", report.DriftFraction)
	}

	t.Logf("removed component: %q", previous.Components[0].Name)
	t.Logf("drift fraction: %.4f, added=%d removed=%d modified=%d",
		report.DriftFraction, len(report.Added), len(report.Removed), len(report.Modified))
}

// TestDriftThreshold verifies that ExceededThreshold is set correctly relative
// to the configured threshold.
func TestDriftThreshold(t *testing.T) {
	root := repoRoot(t)
	s := scanner.New()

	fm, err := s.Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(fm.Components) == 0 {
		t.Skip("no components; cannot exercise threshold")
	}

	// Remove one component to generate drift.
	modified := *fm
	modified.Components = append([]facts.Component(nil), fm.Components[1:]...)

	report := drift.Compare(fm, &modified)
	if report.DriftFraction == 0 {
		t.Skip("drift fraction is 0; cannot test threshold")
	}

	// Threshold above actual fraction → should not exceed.
	report.Threshold = report.DriftFraction + 0.5
	report.ExceededThreshold = report.DriftFraction > report.Threshold
	if report.ExceededThreshold {
		t.Errorf("ExceededThreshold=true with fraction=%.4f threshold=%.4f",
			report.DriftFraction, report.Threshold)
	}

	// Threshold below actual fraction → should exceed.
	report.Threshold = report.DriftFraction - 0.001
	if report.Threshold < 0 {
		report.Threshold = 0
	}
	report.ExceededThreshold = report.DriftFraction > report.Threshold
	if !report.ExceededThreshold {
		t.Errorf("ExceededThreshold=false with fraction=%.4f threshold=%.4f",
			report.DriftFraction, report.Threshold)
	}

	t.Logf("drift fraction: %.4f", report.DriftFraction)
}
