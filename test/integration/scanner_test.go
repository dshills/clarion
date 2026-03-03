//go:build integration

package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/clarion-dev/clarion/internal/facts"
	"github.com/clarion-dev/clarion/internal/scanner"
)

// repoRoot walks up from the working directory until it finds go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (no go.mod in any parent directory)")
		}
		dir = parent
	}
}

// readFileOrSkip reads a file and returns its contents, or skips the test if absent.
func readFileOrSkip(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("cannot read %s: %v", path, err)
	}
	return string(data)
}

// findSpec finds SPEC.md in the repo root or specs/ sub-directory.
func findSpec(root string) string {
	for _, candidate := range []string{
		filepath.Join(root, "SPEC.md"),
		filepath.Join(root, "specs", "SPEC.md"),
	} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

// findPlan finds PLAN.md in the repo root or specs/ sub-directory.
func findPlan(root string) string {
	for _, candidate := range []string{
		filepath.Join(root, "PLAN.md"),
		filepath.Join(root, "specs", "PLAN.md"),
	} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

// TestScanClarionRepo verifies that the scanner produces a valid, populated
// FactModel from the clarion repository itself. No LLM calls are made.
func TestScanClarionRepo(t *testing.T) {
	root := repoRoot(t)
	s := scanner.New()

	fm, err := s.Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	// Schema and project identity.
	if fm.SchemaVersion == "" {
		t.Error("SchemaVersion is empty")
	}
	if fm.SchemaVersion != facts.SchemaV1 {
		t.Errorf("SchemaVersion = %q, want %q", fm.SchemaVersion, facts.SchemaV1)
	}
	if fm.Project.Name == "" {
		t.Error("Project.Name is empty")
	}
	if !strings.Contains(strings.ToLower(fm.Project.Name), "clarion") {
		t.Errorf("Project.Name = %q; want a name containing 'clarion'", fm.Project.Name)
	}
	if fm.Project.GoModule == "" {
		t.Error("Project.GoModule is empty; expected go.mod to be parsed")
	}

	// Components: the scanner should detect at least the cmd/clarion entrypoint
	// plus the internal packages.
	if len(fm.Components) == 0 {
		t.Error("Components is empty; expected at least one component")
	}
	t.Logf("components (%d):", len(fm.Components))
	for _, c := range fm.Components {
		t.Logf("  - %s (score=%.1f)", c.Name, c.ConfidenceScore)
	}

	// Config vars: config.go uses os.Getenv; the scanner should detect at
	// least the CLARION_LLM_PROVIDER and CLARION_LLM_MODEL references.
	if len(fm.Config) == 0 {
		t.Error("Config is empty; expected env var references from config.go")
	}
	t.Logf("config vars (%d):", len(fm.Config))
	for _, cv := range fm.Config {
		t.Logf("  - %s (key=%s)", cv.Name, cv.EnvKey)
	}

	// Validate: every component must have a Name and at least one SourceFile.
	if err := facts.Validate(fm); err != nil {
		t.Errorf("Validate: %v", err)
	}

	// Language detection: Go source should be the dominant language.
	if len(fm.Project.Languages) == 0 {
		t.Error("Languages is empty")
	}
	if fm.Project.Languages[0] != "Go" {
		t.Errorf("dominant language = %q, want %q", fm.Project.Languages[0], "Go")
	}

	t.Logf("project=%q module=%q schema=%s languages=%v",
		fm.Project.Name, fm.Project.GoModule, fm.SchemaVersion, fm.Project.Languages)
	t.Logf("summary: components=%d apis=%d datastores=%d jobs=%d integrations=%d config=%d",
		len(fm.Components), len(fm.APIs), len(fm.Datastores),
		len(fm.Jobs), len(fm.Integrations), len(fm.Config))
}

// TestScanIdempotent verifies that scanning the same repo twice produces
// structurally identical FactModels (same counts and names).
func TestScanIdempotent(t *testing.T) {
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

	if len(fm1.Components) != len(fm2.Components) {
		t.Errorf("Components: scan1=%d scan2=%d; want equal",
			len(fm1.Components), len(fm2.Components))
	}
	if len(fm1.APIs) != len(fm2.APIs) {
		t.Errorf("APIs: scan1=%d scan2=%d; want equal",
			len(fm1.APIs), len(fm2.APIs))
	}
	if len(fm1.Config) != len(fm2.Config) {
		t.Errorf("Config: scan1=%d scan2=%d; want equal",
			len(fm1.Config), len(fm2.Config))
	}
	t.Logf("idempotent: components=%d apis=%d config=%d",
		len(fm1.Components), len(fm1.APIs), len(fm1.Config))
}
