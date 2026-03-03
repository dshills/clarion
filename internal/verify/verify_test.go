package verify

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/clarion-dev/clarion/internal/facts"
)

// makeFactModel builds a FactModel with components at various confidence levels.
func makeFactModel() *facts.FactModel {
	return &facts.FactModel{
		SchemaVersion: facts.SchemaV1,
		Project:       facts.ProjectInfo{Name: "testproject"},
		Components: []facts.Component{
			{
				Name: "MainServer",
				Evidence: facts.Evidence{
					ConfidenceScore: 0.9,
					Inferred:        false,
					SourceFiles:     []string{"cmd/main.go"},
				},
			},
			{
				Name: "WorkerService",
				Evidence: facts.Evidence{
					ConfidenceScore: 0.5,
					Inferred:        true,
					SourceFiles:     []string{"internal/worker/worker.go"},
				},
			},
		},
	}
}

// TestVerifySection_AllHighConfidence verifies that claims matching entries
// with score >= 0.7 result in no FailedClaims and HighConfidence == 100%.
func TestVerifySection_AllHighConfidence(t *testing.T) {
	fm := makeFactModel()
	v := New("/tmp")

	// Only reference the high-confidence component.
	markdown := "MainServer is the primary entry point for all requests."
	report := v.VerifySection("architecture", markdown, fm)

	if len(report.FailedClaims) != 0 {
		t.Errorf("expected no failed claims, got %d: %v", len(report.FailedClaims), report.FailedClaims)
	}
	if report.HighConfidence != 100.0 {
		t.Errorf("expected HighConfidence=100, got %.1f", report.HighConfidence)
	}
}

// TestVerifySection_MediumConfidenceClaim verifies that a claim referencing an
// entry with score in [0.4, 0.7) appears in FailedClaims and MediumConfidence > 0.
func TestVerifySection_MediumConfidenceClaim(t *testing.T) {
	fm := makeFactModel()
	v := New("/tmp")

	// Reference the medium-confidence inferred component.
	markdown := "WorkerService processes jobs asynchronously."
	report := v.VerifySection("architecture", markdown, fm)

	if len(report.FailedClaims) != 1 {
		t.Errorf("expected 1 failed claim, got %d", len(report.FailedClaims))
	}
	if report.MediumConfidence == 0 {
		t.Errorf("expected MediumConfidence > 0, got %.1f", report.MediumConfidence)
	}
	if report.FailedClaims[0].MatchedName != "WorkerService" {
		t.Errorf("expected MatchedName=WorkerService, got %q", report.FailedClaims[0].MatchedName)
	}
}

// TestVerifySection_NoFactModelMatch verifies that a claim with no FactModel
// match appears in FailedClaims with Supported=false and empty MatchedName.
func TestVerifySection_NoFactModelMatch(t *testing.T) {
	fm := makeFactModel()
	v := New("/tmp")

	// Reference something not in the FactModel.
	markdown := "DatabaseProxy manages all SQL connections."
	report := v.VerifySection("architecture", markdown, fm)

	if len(report.FailedClaims) == 0 {
		t.Fatal("expected at least one failed claim")
	}
	found := false
	for _, c := range report.FailedClaims {
		if c.MatchedName == "" && !c.Supported {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected unsupported claim with empty MatchedName, got: %v", report.FailedClaims)
	}
}

// TestVerifySection_EmptyMarkdown verifies that empty markdown produces zero
// percentages and no panic.
func TestVerifySection_EmptyMarkdown(t *testing.T) {
	fm := makeFactModel()
	v := New("/tmp")

	report := v.VerifySection("architecture", "", fm)

	if report.HighConfidence != 0 {
		t.Errorf("expected HighConfidence=0, got %.1f", report.HighConfidence)
	}
	if report.MediumConfidence != 0 {
		t.Errorf("expected MediumConfidence=0, got %.1f", report.MediumConfidence)
	}
	if report.LowConfidence != 0 {
		t.Errorf("expected LowConfidence=0, got %.1f", report.LowConfidence)
	}
	if len(report.FailedClaims) != 0 {
		t.Errorf("expected no failed claims for empty markdown, got %d", len(report.FailedClaims))
	}
}

// TestVerifySection_HeadingsAndCodeBlocksSkipped verifies that headings and
// code block contents are not treated as claims.
func TestVerifySection_HeadingsAndCodeBlocksSkipped(t *testing.T) {
	fm := makeFactModel()
	v := New("/tmp")

	// A heading mentioning MainServer and a code block mentioning WorkerService.
	// Neither should produce claims.
	markdown := "# MainServer Overview\n```go\nWorkerService.Run()\n```"
	report := v.VerifySection("architecture", markdown, fm)

	// No sentences extracted → no claims, no percentages.
	if report.HighConfidence != 0 || report.MediumConfidence != 0 || report.LowConfidence != 0 {
		t.Errorf("expected all zero percentages for heading-only + code markdown")
	}
}

// TestVerifyAll_CreatesTempFiles creates two .md files and verifies that
// VerifyAll returns correct reports with allPassed=false when one has failures.
func TestVerifyAll_CreatesTempFiles(t *testing.T) {
	dir := t.TempDir()

	fm := makeFactModel()

	// File 1: only high-confidence reference.
	clean := "MainServer handles all incoming HTTP requests."
	if err := os.WriteFile(filepath.Join(dir, "clean.md"), []byte(clean), 0o644); err != nil {
		t.Fatalf("write clean.md: %v", err)
	}

	// File 2: reference to something not in the FactModel.
	dirty := "UnknownComponent does mysterious things."
	if err := os.WriteFile(filepath.Join(dir, "dirty.md"), []byte(dirty), 0o644); err != nil {
		t.Fatalf("write dirty.md: %v", err)
	}

	v := New(dir)
	reports, allPassed, err := v.VerifyAll(fm)
	if err != nil {
		t.Fatalf("VerifyAll: %v", err)
	}

	if allPassed {
		t.Error("expected allPassed=false because dirty.md has unsupported claims")
	}

	if len(reports) != 2 {
		t.Errorf("expected 2 reports, got %d", len(reports))
	}

	// Count total failed claims across all reports.
	totalFailed := 0
	for _, r := range reports {
		totalFailed += len(r.FailedClaims)
	}
	if totalFailed == 0 {
		t.Error("expected at least one failed claim across all reports")
	}
}

// TestVerifyAll_EmptyDir verifies that VerifyAll on an empty directory returns
// no reports and allPassed=true.
func TestVerifyAll_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	fm := makeFactModel()

	v := New(dir)
	reports, allPassed, err := v.VerifyAll(fm)
	if err != nil {
		t.Fatalf("VerifyAll: %v", err)
	}

	if !allPassed {
		t.Error("expected allPassed=true for empty dir")
	}
	if len(reports) != 0 {
		t.Errorf("expected 0 reports, got %d", len(reports))
	}
}

// TestExtractSentences_SkipsHeadingsAndCode verifies the internal sentence
// extractor handles headings, code blocks, and empty lines correctly.
func TestExtractSentences_SkipsHeadingsAndCode(t *testing.T) {
	markdown := "# Heading\n\nThis is a sentence.\n```go\ncode here\n```\n- list item"
	sentences := extractSentences(markdown)

	for _, s := range sentences {
		if s == "# Heading" {
			t.Error("heading should be skipped")
		}
		if s == "code here" {
			t.Error("code block content should be skipped")
		}
	}

	found := false
	for _, s := range sentences {
		if s == "This is a sentence." {
			found = true
		}
	}
	if !found {
		t.Errorf("expected sentence to be extracted, got: %v", sentences)
	}
}

// TestContainsCapitalized checks the helper function directly.
func TestContainsCapitalized(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"Hello world", true},
		{"all lowercase", false},
		{"", false},
		{"123 ABC", true},
		{"- item", false},
	}

	for _, tc := range tests {
		got := containsCapitalized(tc.input)
		if got != tc.want {
			t.Errorf("containsCapitalized(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}
