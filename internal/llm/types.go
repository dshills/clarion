package llm

import "context"

// LLMRequest is the input to a single LLM call.
type LLMRequest struct {
	Prompt    string
	MaxTokens int
}

// LLMResponse is the output from a single LLM call.
type LLMResponse struct {
	Text             string
	PromptTokens     int
	CompletionTokens int
	ModelID          string
	LatencyMS        int64
}

// ProviderAdapter abstracts OpenAI and Anthropic behind a common interface.
type ProviderAdapter interface {
	// Name returns the provider name (e.g., "openai", "anthropic").
	Name() string
	// Validate checks that the adapter is properly configured.
	Validate() error
	// Call sends req to the LLM and returns the response.
	Call(ctx context.Context, req LLMRequest) (LLMResponse, error)
}

// Stage identifies the pipeline stage.
type Stage string

const (
	StageSummarize Stage = "summarize"
	StageGenerate  Stage = "generate"
	StageVerify    Stage = "verify"
)

// StageResult holds the output of one pipeline stage.
type StageResult struct {
	Stage            Stage
	Response         LLMResponse
	CumulativeTokens int
}

// PipelineStage is a stage definition in a Pipeline.
type PipelineStage struct {
	Name     Stage
	Prompt   string
	Required bool // if true, budget exhaustion before this stage is fatal
}
