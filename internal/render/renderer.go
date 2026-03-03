// Package render handles writing documentation files to the output directory.
package render

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/clarion-dev/clarion/internal/facts"
)

// Renderer writes documentation files to an output directory.
type Renderer struct {
	outputDir string
	jsonMode  bool
}

// New creates a Renderer targeting the given output directory.
func New(outputDir string, jsonMode bool) *Renderer {
	return &Renderer{outputDir: outputDir, jsonMode: jsonMode}
}

// WriteMarkdown writes content to outputDir/filename.
func (r *Renderer) WriteMarkdown(filename, content string) error {
	if err := os.MkdirAll(r.outputDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	path := filepath.Join(r.outputDir, filename)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", filename, err)
	}
	if !r.jsonMode {
		fmt.Printf("Wrote: %s\n", path)
	}
	return nil
}

// WriteMermaid extracts the first mermaid block from markdown and writes it
// to outputDir/diagrams/filename.
func (r *Renderer) WriteMermaid(filename, markdown string) error {
	diagram, ok := ExtractMermaid(markdown)
	if !ok {
		return nil // no mermaid block found; not an error
	}
	diagDir := filepath.Join(r.outputDir, "diagrams")
	if err := os.MkdirAll(diagDir, 0o755); err != nil {
		return fmt.Errorf("create diagrams dir: %w", err)
	}
	path := filepath.Join(diagDir, filename)
	if err := os.WriteFile(path, []byte(diagram), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", filename, err)
	}
	if !r.jsonMode {
		fmt.Printf("Wrote: %s\n", path)
	}
	return nil
}

// WriteFactModel serializes fm to outputDir/clarion-meta.json.
func (r *Renderer) WriteFactModel(fm *facts.FactModel) error {
	if err := os.MkdirAll(r.outputDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	path := filepath.Join(r.outputDir, "clarion-meta.json")
	return facts.Save(path, fm)
}

// WriteJSON marshals result and prints it to stdout (for --json mode).
func (r *Renderer) WriteJSON(result any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
