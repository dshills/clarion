package generator

import (
	"context"
	"strings"
	"testing"

	"github.com/clarion-dev/clarion/internal/llm"
)

// capturingAdapter records every prompt sent to it.
type capturingAdapter struct {
	prompts []string
}

func (c *capturingAdapter) Name() string    { return "capture" }
func (c *capturingAdapter) Validate() error { return nil }
func (c *capturingAdapter) Call(_ context.Context, req llm.LLMRequest) (llm.LLMResponse, error) {
	c.prompts = append(c.prompts, req.Prompt)
	return llm.LLMResponse{Text: "test output", PromptTokens: 10, CompletionTokens: 5}, nil
}

// TestNoRawRepoInPrompt asserts that no raw Go source code is included in LLM
// prompts. Prompts must only contain FactModel JSON, SPEC text, and PLAN text.
func TestNoRawRepoInPrompt(t *testing.T) {
	// goSourcePatterns are fragments that would only appear if raw Go source file
	// content (not metadata) was injected into the prompt. These patterns are
	// chosen to avoid false-positives from English prose in prompt templates
	// (e.g. "package main files" is valid English; "\npackage main\n" is source).
	goSourcePatterns := []string{
		"\npackage main\n",   // standalone Go package declaration
		"func main() {",     // main function body
		"func main(){\n",    // main function body (no space variant)
		"\nimport (\n",      // import block
		"var _ = ",          // blank identifier assignment
	}

	adapter := &capturingAdapter{}
	budget := llm.NewBudgetTracker(100000)
	pipeline := llm.NewPipeline(adapter, budget, false)

	gen, err := New(pipeline)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	fm := makeFactModel()
	sections := []string{"architecture", "api", "data-model", "runbook"}
	for _, section := range sections {
		_, err := gen.GenerateSection(context.Background(), section, fm, "spec text here", "")
		if err != nil {
			t.Fatalf("GenerateSection(%s): %v", section, err)
		}
	}

	for i, prompt := range adapter.prompts {
		for _, pattern := range goSourcePatterns {
			if strings.Contains(prompt, pattern) {
				t.Errorf("prompt[%d] contains raw Go source pattern %q; prompts must only contain FactModel JSON, spec, and plan text", i, pattern)
			}
		}
	}
}
