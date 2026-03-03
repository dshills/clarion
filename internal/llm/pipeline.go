package llm

import (
	"context"
	"fmt"
	"log"
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

// Run executes the pipeline stages in order, respecting the token budget.
// If a required stage cannot be afforded, it returns ErrBudgetExhausted.
// If an optional stage is skipped, it returns ErrBudgetSkipped with partial results.
func (p *Pipeline) Run(ctx context.Context, stages []PipelineStage) ([]StageResult, error) {
	if p.budget.Remaining() == 0 {
		return nil, fmt.Errorf("%w: budget is 0 before any stage", ErrBudgetExhausted)
	}

	var results []StageResult

	for _, stage := range stages {
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
