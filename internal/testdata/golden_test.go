package testdata_test

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/clarion-dev/clarion/internal/drift"
	"github.com/clarion-dev/clarion/internal/facts"
	"github.com/clarion-dev/clarion/internal/generator"
	"github.com/clarion-dev/clarion/internal/render"
	"github.com/clarion-dev/clarion/internal/verify"
)

var update = flag.Bool("update", false, "Update golden files")

func goldenPath(name string) string {
	return filepath.Join("golden", name)
}

func readGolden(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(goldenPath(name))
	if err != nil {
		t.Fatalf("read golden %s: %v", name, err)
	}
	return string(data)
}

func writeGolden(t *testing.T, name, content string) {
	t.Helper()
	if err := os.MkdirAll("golden", 0o755); err != nil {
		t.Fatalf("mkdir golden: %v", err)
	}
	if err := os.WriteFile(goldenPath(name), []byte(content), 0o644); err != nil {
		t.Fatalf("write golden %s: %v", name, err)
	}
}

// fixtureFactModel returns a minimal FactModel for golden file tests.
func fixtureFactModel() *facts.FactModel {
	return &facts.FactModel{
		SchemaVersion: facts.SchemaV1,
		Project: facts.ProjectInfo{
			Name:      "myapp",
			GoModule:  "github.com/example/myapp",
			Languages: []string{"Go"},
		},
		Components: []facts.Component{
			{Name: "api-server", Evidence: facts.Evidence{
				SourceFiles:     []string{"cmd/server/main.go"},
				ConfidenceScore: 0.9,
			}},
		},
		APIs: []facts.APIEndpoint{
			{Name: "GET /users", Method: "GET", Route: "/users", Handler: "listUsers",
				Evidence: facts.Evidence{SourceFiles: []string{"api/users.go"}, ConfidenceScore: 0.9}},
			{Name: "POST /users", Method: "POST", Route: "/users", Handler: "createUser",
				Evidence: facts.Evidence{SourceFiles: []string{"api/users.go"}, ConfidenceScore: 0.5, Inferred: true}},
		},
		Datastores: []facts.Datastore{
			{Name: "postgres-datastore", Driver: "postgres",
				Evidence: facts.Evidence{SourceFiles: []string{"db/db.go"}, ConfidenceScore: 0.9}},
		},
		Config: []facts.ConfigVar{
			{Name: "DATABASE_URL", EnvKey: "DATABASE_URL",
				Evidence: facts.Evidence{SourceFiles: []string{"config.go"}, ConfidenceScore: 0.9}},
		},
	}
}

func TestGoldenInferredMarkers(t *testing.T) {
	fm := fixtureFactModel()

	input := `# API Reference

The api-server component handles HTTP requests.
GET /users lists all users.
POST /users creates a new user.
postgres-datastore stores user records.
`
	got := generator.ApplyInferredMarkers(input, fm)

	if *update {
		writeGolden(t, "markers.md", got)
		return
	}
	want := readGolden(t, "markers.md")
	if got != want {
		t.Errorf("markers output mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestGoldenExtractMermaid(t *testing.T) {
	input := `# Architecture

Some text.

` + "```mermaid\ngraph TD\n  A --> B\n  B --> C\n```" + `

More text.
`
	got, ok := render.ExtractMermaid(input)
	if !ok {
		t.Fatal("ExtractMermaid returned ok=false")
	}

	if *update {
		writeGolden(t, "component.mmd", got)
		return
	}
	want := readGolden(t, "component.mmd")
	if strings.TrimSpace(got) != strings.TrimSpace(want) {
		t.Errorf("mermaid mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestGoldenDriftMarkdown(t *testing.T) {
	report := drift.DriftReport{
		DriftFraction:     0.25,
		Threshold:         0.20,
		ExceededThreshold: true,
		Added: []drift.DriftEntry{
			{Type: "api", Name: "GET /users", Change: "added"},
		},
		Removed: []drift.DriftEntry{
			{Type: "datastore", Name: "redis-datastore", Change: "removed"},
		},
	}

	got := report.Markdown()

	if *update {
		writeGolden(t, "drift-report.md", got)
		return
	}
	want := readGolden(t, "drift-report.md")
	if strings.TrimSpace(got) != strings.TrimSpace(want) {
		t.Errorf("drift markdown mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestGoldenVerifySection(t *testing.T) {
	fm := fixtureFactModel()
	v := verify.New(t.TempDir()) // outputDir not used by VerifySection

	md := `# Architecture
The api-server serves HTTP traffic.
GET /users is the main endpoint.
postgres-datastore stores all data.
`
	report := v.VerifySection("architecture", md, fm)

	// Build a simple text summary of the report.
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Section: %s\n", report.Section))
	sb.WriteString(fmt.Sprintf("High: %.0f%%\n", report.HighConfidence))
	sb.WriteString(fmt.Sprintf("Medium: %.0f%%\n", report.MediumConfidence))
	sb.WriteString(fmt.Sprintf("Low: %.0f%%\n", report.LowConfidence))
	sb.WriteString(fmt.Sprintf("FailedClaims: %d\n", len(report.FailedClaims)))
	got := sb.String()

	if *update {
		writeGolden(t, "verify-report.txt", got)
		return
	}
	want := readGolden(t, "verify-report.txt")
	if strings.TrimSpace(got) != strings.TrimSpace(want) {
		t.Errorf("verify report mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}
