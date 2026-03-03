package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/clarion-dev/clarion/internal/facts"
)

// makeFactModel builds a minimal FactModel for render tests.
func makeFactModel() *facts.FactModel {
	return &facts.FactModel{
		SchemaVersion: facts.SchemaV1,
		GeneratedAt:   time.Now(),
		Project:       facts.ProjectInfo{Name: "testproject", RootPath: "/tmp/test"},
	}
}

// TestWriteMarkdown_CreatesFile verifies that WriteMarkdown creates the file
// in the output directory with the given content.
func TestWriteMarkdown_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	r := New(dir, false)

	content := "# Architecture\n\nThis is a test."
	if err := r.WriteMarkdown("architecture.md", content); err != nil {
		t.Fatalf("WriteMarkdown: %v", err)
	}

	path := filepath.Join(dir, "architecture.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if string(data) != content {
		t.Errorf("file content mismatch: got %q, want %q", string(data), content)
	}
}

// TestWriteMarkdown_CreatesOutputDir verifies that the output directory is
// created if it does not exist.
func TestWriteMarkdown_CreatesOutputDir(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "nested", "output")
	r := New(dir, false)

	if err := r.WriteMarkdown("test.md", "content"); err != nil {
		t.Fatalf("WriteMarkdown: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "test.md")); err != nil {
		t.Errorf("file not created: %v", err)
	}
}

// TestWriteMermaid_WritesDiagramSubdir verifies that WriteMermaid writes to
// the diagrams/ subdirectory when a mermaid block is present.
func TestWriteMermaid_WritesDiagramSubdir(t *testing.T) {
	dir := t.TempDir()
	r := New(dir, false)

	markdown := "Some text.\n```mermaid\ngraph TD\n    A --> B\n```\nMore text."
	if err := r.WriteMermaid("component.mmd", markdown); err != nil {
		t.Fatalf("WriteMermaid: %v", err)
	}

	path := filepath.Join(dir, "diagrams", "component.mmd")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if !strings.Contains(string(data), "graph TD") {
		t.Errorf("expected mermaid diagram content, got: %q", string(data))
	}
}

// TestWriteMermaid_NoOpWhenNoMermaid verifies that WriteMermaid is a no-op
// when no mermaid block is present in the markdown.
func TestWriteMermaid_NoOpWhenNoMermaid(t *testing.T) {
	dir := t.TempDir()
	r := New(dir, false)

	markdown := "# No diagrams here.\n\nJust plain text."
	if err := r.WriteMermaid("component.mmd", markdown); err != nil {
		t.Fatalf("WriteMermaid: %v", err)
	}

	// The diagrams directory should not be created.
	diagDir := filepath.Join(dir, "diagrams")
	if _, err := os.Stat(diagDir); !os.IsNotExist(err) {
		t.Errorf("expected diagrams dir to not exist, but it does")
	}
}

// TestWriteFactModel_WritesJSON verifies that WriteFactModel creates
// clarion-meta.json in the output directory.
func TestWriteFactModel_WritesJSON(t *testing.T) {
	dir := t.TempDir()
	r := New(dir, false)

	fm := makeFactModel()
	if err := r.WriteFactModel(fm); err != nil {
		t.Fatalf("WriteFactModel: %v", err)
	}

	path := filepath.Join(dir, "clarion-meta.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("clarion-meta.json not created: %v", err)
	}

	loaded, err := facts.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.Project.Name != fm.Project.Name {
		t.Errorf("project name mismatch: got %q, want %q", loaded.Project.Name, fm.Project.Name)
	}
}

// TestExtractMermaid_FindsFirstBlock verifies that ExtractMermaid returns the
// content of the first mermaid block.
func TestExtractMermaid_FindsFirstBlock(t *testing.T) {
	markdown := "Text before.\n```mermaid\ngraph TD\n    A --> B\n```\nText after."
	got, ok := ExtractMermaid(markdown)
	if !ok {
		t.Fatal("expected mermaid block to be found")
	}
	if !strings.Contains(got, "graph TD") {
		t.Errorf("expected 'graph TD' in extracted content, got: %q", got)
	}
}

// TestExtractMermaid_ReturnsFalseWhenAbsent verifies that ExtractMermaid
// returns false when no mermaid block is present.
func TestExtractMermaid_ReturnsFalseWhenAbsent(t *testing.T) {
	markdown := "# No mermaid here\n\nJust text."
	_, ok := ExtractMermaid(markdown)
	if ok {
		t.Error("expected false for markdown without mermaid block")
	}
}

// TestExtractMermaid_MultipleBlocksReturnsFirst verifies that ExtractMermaid
// returns the first mermaid block when multiple are present.
func TestExtractMermaid_MultipleBlocksReturnsFirst(t *testing.T) {
	markdown := "```mermaid\ngraph TD\n    First --> A\n```\n\n```mermaid\nerDiagram\n    Second\n```"
	got, ok := ExtractMermaid(markdown)
	if !ok {
		t.Fatal("expected mermaid block to be found")
	}
	if !strings.Contains(got, "First") {
		t.Errorf("expected first block content, got: %q", got)
	}
	if strings.Contains(got, "Second") {
		t.Errorf("expected only first block content, but got second block too: %q", got)
	}
}

// TestExtractMermaid_UnclosedBlock verifies that an unclosed mermaid fence
// returns false.
func TestExtractMermaid_UnclosedBlock(t *testing.T) {
	markdown := "```mermaid\ngraph TD\n    A --> B\n"
	_, ok := ExtractMermaid(markdown)
	if ok {
		t.Error("expected false for unclosed mermaid block")
	}
}
