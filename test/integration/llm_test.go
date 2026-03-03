//go:build integration

package integration_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/clarion-dev/clarion/internal/llm"
)

// runProviderSmoke sends a minimal prompt through a configured pipeline and
// validates the response shape. It does not assert on the text content since
// LLM output is non-deterministic.
func runProviderSmoke(t *testing.T) {
	t.Helper()

	cfg, err := llm.LoadConfig()
	if err != nil {
		t.Skipf("LLM config not available: %v", err)
	}

	adapter, err := llm.NewAdapter(cfg)
	if err != nil {
		t.Skipf("adapter unavailable for provider %q: %v", cfg.Provider, err)
	}

	budget := llm.NewBudgetTracker(10000)
	pipeline := llm.NewPipeline(adapter, budget, false)

	ctx := context.Background()
	results, err := pipeline.Run(ctx, []llm.PipelineStage{
		{
			Name:     llm.StageGenerate,
			Prompt:   "Respond with exactly: OK",
			Required: true,
		},
	})
	if err != nil {
		t.Fatalf("pipeline.Run: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("pipeline returned no results")
	}

	resp := results[0].Response
	if strings.TrimSpace(resp.Text) == "" {
		t.Error("response text is empty")
	}
	if resp.PromptTokens <= 0 {
		t.Errorf("PromptTokens = %d, want > 0", resp.PromptTokens)
	}
	if resp.CompletionTokens <= 0 {
		t.Errorf("CompletionTokens = %d, want > 0", resp.CompletionTokens)
	}
	if resp.ModelID == "" {
		t.Error("ModelID is empty")
	}
	t.Logf("provider=%s model=%s text=%q prompt_tokens=%d completion_tokens=%d latency=%dms",
		cfg.Provider, resp.ModelID, resp.Text,
		resp.PromptTokens, resp.CompletionTokens, resp.LatencyMS)
}

// TestLLMProviderOpenAI verifies the OpenAI adapter makes a real API call and
// returns a structurally valid response. Uses gpt-4o-mini to minimise cost.
func TestLLMProviderOpenAI(t *testing.T) {
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("OPENAI_API_KEY not set")
	}
	t.Setenv("CLARION_LLM_PROVIDER", "openai")
	t.Setenv("CLARION_LLM_MODEL", "gpt-4o-mini")
	runProviderSmoke(t)
}

// TestLLMProviderAnthropic verifies the Anthropic adapter makes a real API
// call and returns a structurally valid response. Uses claude-haiku-4-5 to
// minimise cost.
func TestLLMProviderAnthropic(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("ANTHROPIC_API_KEY not set")
	}
	t.Setenv("CLARION_LLM_PROVIDER", "anthropic")
	t.Setenv("CLARION_LLM_MODEL", "claude-haiku-4-5-20251001")
	runProviderSmoke(t)
}

// TestLLMBudgetEnforced verifies that the budget tracker correctly terminates
// the pipeline when tokens are exhausted. Uses a mock adapter — no real calls.
func TestLLMBudgetEnforced(t *testing.T) {
	adapter := &llm.MockAdapter{
		Responses: map[string]llm.LLMResponse{
			"": {Text: "done", PromptTokens: 5000, CompletionTokens: 5000},
		},
	}
	// Budget of 1000 tokens; mock returns 10000 → second call must be rejected.
	budget := llm.NewBudgetTracker(1000)
	pipeline := llm.NewPipeline(adapter, budget, false)

	ctx := context.Background()
	_, err := pipeline.Run(ctx, []llm.PipelineStage{
		{Name: llm.StageGenerate, Prompt: "first call", Required: true},
		{Name: llm.StageVerify, Prompt: "second call", Required: true},
	})
	if err == nil {
		t.Fatal("expected budget error, got nil")
	}
	t.Logf("budget error (expected): %v", err)
}
