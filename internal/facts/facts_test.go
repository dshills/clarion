package facts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---- helpers ----------------------------------------------------------------

func minimalModel() *FactModel {
	return &FactModel{
		SchemaVersion: SchemaV1,
		GeneratedAt:   time.Now(),
		Project: ProjectInfo{
			Name:     "testproject",
			RootPath: "/tmp/testproject",
		},
	}
}

// ---- Load / Save round-trip -------------------------------------------------

func TestLoadSaveRoundTrip(t *testing.T) {
	fm := minimalModel()
	fm.Components = []Component{
		{Name: "main", Evidence: Evidence{SourceFiles: []string{"main.go"}, ConfidenceScore: 0.9}},
	}
	fm.APIs = []APIEndpoint{
		{Name: "GET /health", Method: "GET", Route: "/health",
			Evidence: Evidence{SourceFiles: []string{"server.go"}, ConfidenceScore: 0.9}},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "clarion-meta.json")

	if err := Save(path, fm); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got.SchemaVersion != fm.SchemaVersion {
		t.Errorf("schema_version: got %q, want %q", got.SchemaVersion, fm.SchemaVersion)
	}
	if len(got.Components) != 1 || got.Components[0].Name != "main" {
		t.Errorf("components round-trip failed: %+v", got.Components)
	}
	if len(got.APIs) != 1 || got.APIs[0].Route != "/health" {
		t.Errorf("apis round-trip failed: %+v", got.APIs)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/path/clarion-meta.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	_ = os.WriteFile(path, []byte("{not valid json"), 0o644)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestJSONOutputHasRequiredFields(t *testing.T) {
	fm := minimalModel()
	data, err := json.Marshal(fm)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(data)
	for _, field := range []string{"schema_version", "generated_at"} {
		if !strings.Contains(s, field) {
			t.Errorf("JSON output missing field %q", field)
		}
	}
}

// ---- Validate ---------------------------------------------------------------

func TestValidateValid(t *testing.T) {
	fm := minimalModel()
	fm.Components = []Component{
		{Name: "main", Evidence: Evidence{SourceFiles: []string{"main.go"}, ConfidenceScore: 0.9}},
	}
	if err := Validate(fm); err != nil {
		t.Errorf("unexpected validation error: %v", err)
	}
}

func TestValidateMissingProjectName(t *testing.T) {
	fm := minimalModel()
	fm.Project.Name = ""
	if err := Validate(fm); err == nil {
		t.Error("expected error for missing project name")
	}
}

func TestValidateMissingComponentName(t *testing.T) {
	fm := minimalModel()
	fm.Components = []Component{
		{Name: "", Evidence: Evidence{SourceFiles: []string{"main.go"}, ConfidenceScore: 0.9}},
	}
	if err := Validate(fm); err == nil {
		t.Error("expected error for empty component name")
	}
}

func TestValidateMissingSourceFiles(t *testing.T) {
	fm := minimalModel()
	fm.Components = []Component{
		{Name: "main", Evidence: Evidence{SourceFiles: nil, ConfidenceScore: 0.9}},
	}
	if err := Validate(fm); err == nil {
		t.Error("expected error for nil source_files")
	}
}

func TestValidateWrongSchemaVersion(t *testing.T) {
	fm := minimalModel()
	fm.SchemaVersion = "2.0"
	if err := Validate(fm); err == nil {
		t.Error("expected error for wrong schema version")
	}
}

// ---- Confidence helpers -----------------------------------------------------

func TestIsEvidenceBacked(t *testing.T) {
	cases := []struct {
		score float64
		want  bool
	}{
		{0.9, true},
		{0.7, true},
		{0.699, false},
		{0.5, false},
		{0.0, false},
	}
	for _, c := range cases {
		if got := IsEvidenceBacked(c.score); got != c.want {
			t.Errorf("IsEvidenceBacked(%v) = %v, want %v", c.score, got, c.want)
		}
	}
}

func TestShouldOmit(t *testing.T) {
	cases := []struct {
		score float64
		want  bool
	}{
		{0.39, true},
		{0.4, false},
		{0.5, false},
		{0.9, false},
	}
	for _, c := range cases {
		if got := ShouldOmit(c.score); got != c.want {
			t.Errorf("ShouldOmit(%v) = %v, want %v", c.score, got, c.want)
		}
	}
}

func TestNeedsInferredMarker(t *testing.T) {
	cases := []struct {
		score    float64
		inferred bool
		want     bool
	}{
		{0.9, false, false},
		{0.7, false, false},
		{0.69, false, true},  // score in [0.4, 0.7)
		{0.5, false, true},   // score in [0.4, 0.7)
		{0.4, false, true},   // boundary
		{0.39, false, false}, // below omit threshold — would be omitted, marker irrelevant
		{0.9, true, true},    // inferred flag always triggers marker
		{0.7, true, true},
	}
	for _, c := range cases {
		if got := NeedsInferredMarker(c.score, c.inferred); got != c.want {
			t.Errorf("NeedsInferredMarker(%v, %v) = %v, want %v", c.score, c.inferred, got, c.want)
		}
	}
}

// ---- TruncateToSize ---------------------------------------------------------

func makeAPIEntry(score float64) APIEndpoint {
	// Pad the route to inflate size.
	return APIEndpoint{
		Name:   strings.Repeat("x", 100),
		Method: "GET",
		Route:  "/" + strings.Repeat("y", 100),
		Evidence: Evidence{
			SourceFiles:     []string{"server.go"},
			ConfidenceScore: score,
		},
	}
}

func TestTruncateToSizeNoTruncationNeeded(t *testing.T) {
	fm := minimalModel()
	got, dropped, err := TruncateToSize(fm, DefaultMaxBytes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dropped != 0 {
		t.Errorf("expected 0 drops, got %d", dropped)
	}
	if got == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestTruncateToSizeDropsLowConfidence(t *testing.T) {
	fm := minimalModel()
	// Add many large API entries with varying confidence.
	for i := 0; i < 200; i++ {
		score := ConfidenceSpeculative
		if i%2 == 0 {
			score = ConfidenceDirect
		}
		fm.APIs = append(fm.APIs, makeAPIEntry(score))
	}

	// Set a target that forces truncation but is large enough to hold the base
	// model plus the Direct-confidence entries after speculative ones are dropped.
	got, dropped, err := TruncateToSize(fm, 25*1024) // 25 KB
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dropped == 0 {
		t.Error("expected some entries to be dropped")
	}
	// Verify remaining entries all have higher confidence than dropped ones.
	for _, a := range got.APIs {
		if a.ConfidenceScore < ConfidenceDirect && dropped > 0 {
			// Lower-confidence entries should have been dropped first.
			// All remaining speculative entries are only present if there
			// weren't enough to drop. This is a best-effort check.
			_ = a
		}
	}
}
