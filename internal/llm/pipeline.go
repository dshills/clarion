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

// rawSourceIndicators are Go source-code fragments that must never appear in
// LLM prompts (SPEC.md §9). Prompts may only contain FactModel JSON, SPEC.md
// text, and PLAN.md text — never raw repository file content.
var rawSourceIndicators = []string{
	"\npackage main\n",
	"func main() {",
	"\nimport (\n",
}

// validatePromptContent enforces SPEC.md §9: LLM prompts must never contain
// raw Go source code — only FactModel JSON, SPEC.md, and PLAN.md text.
func validatePromptContent(prompt string) error {
	for _, indicator := range rawSourceIndicators {
		if strings.Contains(prompt, indicator) {
			return fmt.Errorf("prompt contains raw Go source code (indicator %q); see SPEC.md §9", indicator)
		}
	}
	return nil
}

// Run executes the pipeline stages in order, respecting the token budget.
// If a required stage cannot be afforded, it returns ErrBudgetExhausted.
// If an optional stage is skipped, it returns ErrBudgetSkipped with partial results.
func (p *Pipeline) Run(ctx context.Context, stages []PipelineStage) ([]StageResult, error) {
	if p.budget.Remaining() == 0 {
		return nil, fmt.Errorf("%w: budget is 0 before any stage", ErrBudgetExhausted)
	}

	var results []StageResult

	for _, stage := range stages {
		if err := validatePromptContent(stage.Prompt); err != nil {
			return results, err
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
