package llm

import (
	"context"
	"fmt"
	"log"
	"strings"
)

// Pipeline runs a sequence of LLM stages within a token budget.
type Pipeline struct {
	adapter ProviderAdapter
	budget  *BudgetTracker
	verbose bool
}

// NewPipeline creates a Pipeline with the given adapter, budget, and verbosity.
func NewPipeline(adapter ProviderAdapter, budget *BudgetTracker, verbose bool) *Pipeline {
	return &Pipeline{adapter: adapter, budget: budget, verbose: verbose}
}

// maxLeadingLines is the maximum number of lines scanned when checking
// whether a prompt begins with a Go package declaration. Valid prompts
// from the generator always start within the first few lines; raw source
// files always open with "package <ident>" as their first non-blank line.
const maxLeadingLines = 5

// promptIsRawGoSource returns true when the prompt's first non-blank line
// is a bare Go package declaration ("package <ident>", exactly two tokens).
//
// Valid prompt sources never begin this way:
//   - FactModel JSON starts with "{"
//   - SPEC.md / PLAN.md start with a title or prose sentence
//   - Generator template output starts with "You are a …"
//
// English prose like "package main files" has three or more tokens and is
// not flagged. This check is structural, not pattern-string matching.
func promptIsRawGoSource(prompt string) bool {
	for _, line := range strings.SplitN(strings.TrimSpace(prompt), "\n", maxLeadingLines) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		// Exactly two tokens where first is "package" → Go package declaration.
		return len(parts) == 2 && parts[0] == "package"
	}
	return false
}

// Run executes the pipeline stages in order, respecting the token budget.
// If a required stage cannot be afforded, it returns ErrBudgetExhausted.
// If an optional stage is skipped, it returns ErrBudgetSkipped with partial results.
//
// Per SPEC.md §9, this method rejects any prompt whose first non-blank line
// is a bare Go package declaration, which is the characteristic signature of
// raw source file content. Valid inputs (FactModel JSON, SPEC.md, PLAN.md)
// never start with "package <ident>".
func (p *Pipeline) Run(ctx context.Context, stages []PipelineStage) ([]StageResult, error) {
	if p.budget.Remaining() == 0 {
		return nil, fmt.Errorf("%w: budget is 0 before any stage", ErrBudgetExhausted)
	}

	var results []StageResult

	for _, stage := range stages {
		if promptIsRawGoSource(stage.Prompt) {
			return results, fmt.Errorf("SPEC.md §9: stage %q prompt appears to be raw Go source; "+
				"prompts must contain only FactModel JSON, SPEC.md, or PLAN.md text", stage.Name)
		}

		estimated := EstimateTokens(stage.Prompt)

		if !p.budget.CanAfford(estimated) {
			if stage.Required {
				log.Printf("WARN: Token budget too small for required stage %s. Increase CLARION_LLM_TOKEN_BUDGET.", stage.Name)
				return results, ErrBudgetExhausted
			}
			log.Printf("WARN: Token budget exceeded: %d/%d tokens. Skipping stage %s and all remaining stages.",
				p.budget.Used(), p.budget.limit, stage.Name)
			return results, ErrBudgetSkipped
		}

		maxTokens := p.budget.Remaining()
		resp, err := p.adapter.Call(ctx, LLMRequest{
			Prompt:    stage.Prompt,
			MaxTokens: maxTokens,
		})
		if err != nil {
			return results, fmt.Errorf("stage %s: %w", stage.Name, err)
		}

		actual := resp.PromptTokens + resp.CompletionTokens
		p.budget.Record(actual)

		if p.verbose {
			cost := float64(actual) * 0.01 / 1000
			log.Printf("stage=%s tokens=%d cumulative=%d cost=$%.4f latency=%dms",
				stage.Name, actual, p.budget.Used(), cost, resp.LatencyMS)
		}

		results = append(results, StageResult{
			Stage:            stage.Name,
			Response:         resp,
			CumulativeTokens: p.budget.Used(),
		})
	}

	return results, nil
}
