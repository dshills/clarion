// Package generator renders documentation sections from a FactModel via an LLM pipeline.
package generator

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"text/template"

	"github.com/clarion-dev/clarion/internal/facts"
	"github.com/clarion-dev/clarion/internal/llm"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

// Generator produces documentation sections by rendering prompt templates
// and calling the LLM pipeline.
type Generator struct {
	pipeline  *llm.Pipeline
	templates *template.Template
}

// New creates a Generator backed by the given pipeline.
func New(pipeline *llm.Pipeline) (*Generator, error) {
	tmpl, err := template.ParseFS(templateFS, "templates/*.tmpl")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}
	return &Generator{pipeline: pipeline, templates: tmpl}, nil
}

// templateData is the data passed to each prompt template.
//
// SPEC.md §9 — LLMs must operate only on FactModel JSON, SPEC.md contents,
// and PLAN.md contents. Never raw repository text. This struct is the sole
// source of template inputs, enforcing that constraint by construction.
// The scanner never reads file content; it only extracts metadata (names,
// paths, confidence scores). TestNoRawRepoInPrompt verifies this invariant.
type templateData struct {
	FactModel string // JSON-encoded FactModel (metadata only, no file content)
	Spec      string // contents of SPEC.md
	Plan      string // contents of PLAN.md (empty if absent)
}

// GenerateSection renders the named template and calls the LLM pipeline.
// section must be one of: "architecture", "api", "data-model", "runbook".
func (g *Generator) GenerateSection(ctx context.Context, section string, fm *facts.FactModel, spec, plan string) (string, error) {
	tmplName := section + ".tmpl"

	fmJSON, err := json.MarshalIndent(fm, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal fact model: %w", err)
	}

	data := templateData{
		FactModel: string(fmJSON),
		Spec:      spec,
		Plan:      plan,
	}

	var buf bytes.Buffer
	if err := g.templates.ExecuteTemplate(&buf, tmplName, data); err != nil {
		return "", fmt.Errorf("render template %s: %w", tmplName, err)
	}
	prompt := buf.String()

	stages := []llm.PipelineStage{
		{Name: llm.StageGenerate, Prompt: prompt, Required: true},
	}

	results, err := g.pipeline.Run(ctx, stages)
	if err != nil {
		return "", fmt.Errorf("pipeline: %w", err)
	}
	if len(results) == 0 {
		return "", fmt.Errorf("pipeline returned no results")
	}

	text := results[0].Response.Text
	return ApplyInferredMarkers(text, fm), nil
}

// GenerateAll generates all four documentation sections.
func (g *Generator) GenerateAll(ctx context.Context, fm *facts.FactModel, spec, plan string) (map[string]string, error) {
	sections := []string{"architecture", "api", "data-model", "runbook"}
	results := make(map[string]string, len(sections))

	for _, section := range sections {
		text, err := g.GenerateSection(ctx, section, fm, spec, plan)
		if err != nil {
			return results, fmt.Errorf("section %s: %w", section, err)
		}
		results[section] = text
	}
	return results, nil
}
