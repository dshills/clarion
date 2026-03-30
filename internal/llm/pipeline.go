package llm

import (
	"context"
	"fmt"
	"log"
)

// defaultMaxOutputTokensPerCall caps the max_tokens sent to the provider on
// any single call. Frontier models (gpt-4o, claude-sonnet-4-*, gemini-2.*)
// support 16384+ completion tokens. Override via CLARION_LLM_MAX_OUTPUT_TOKENS.
const defaultMaxOutputTokensPerCall = 16384

// Pipeline runs a sequence of LLM stages within a token budget.
type Pipeline struct {
	adapter         ProviderAdapter
	budget          *BudgetTracker
	verbose         bool
	maxOutputTokens int
}

// NewPipeline creates a Pipeline with the given adapter, budget, and verbosity.
func NewPipeline(adapter ProviderAdapter, budget *BudgetTracker, verbose bool) *Pipeline {
	return &Pipeline{adapter: adapter, budget: budget, verbose: verbose, maxOutputTokens: defaultMaxOutputTokensPerCall}
}

// NewPipelineWithConfig creates a Pipeline using the full Config for output token limits.
func NewPipelineWithConfig(adapter ProviderAdapter, budget *BudgetTracker, verbose bool, cfg Config) *Pipeline {
	maxOut := cfg.MaxOutputTokens
	if maxOut <= 0 {
		maxOut = defaultMaxOutputTokensPerCall
	}
	return &Pipeline{adapter: adapter, budget: budget, verbose: verbose, maxOutputTokens: maxOut}
}

// Run executes the pipeline stages in order, respecting the token budget.
// If a required stage cannot be afforded, it returns ErrBudgetExhausted.
// If an optional stage is skipped, it returns ErrBudgetSkipped with partial results.
// Each stage prompt is validated by enforceSpec9 (see spec9_guard.go) before
// being sent to the LLM provider.
func (p *Pipeline) Run(ctx context.Context, stages []PipelineStage) ([]StageResult, error) {
	if p.budget.Remaining() == 0 {
		return nil, fmt.Errorf("%w: budget is 0 before any stage", ErrBudgetExhausted)
	}

	var results []StageResult

	for _, stage := range stages {
		if err := enforceSpec9(stage.Name, stage.Prompt); err != nil {
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
		if maxTokens > p.maxOutputTokens {
			maxTokens = p.maxOutputTokens
		}
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
