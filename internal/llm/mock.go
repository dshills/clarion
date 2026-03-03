package llm

import (
	"context"
	"strings"
)

// MockAdapter is a deterministic ProviderAdapter for testing.
// Responses are keyed by exact stage name; a "" key is the fallback.
type MockAdapter struct {
	// Responses maps stage name (or prompt substring) to a canned LLMResponse.
	Responses map[string]LLMResponse
	// CallCount is incremented on each Call invocation.
	CallCount int
}

func (m *MockAdapter) Name() string    { return "mock" }
func (m *MockAdapter) Validate() error { return nil }

func (m *MockAdapter) Call(_ context.Context, req LLMRequest) (LLMResponse, error) {
	m.CallCount++
	// Check for a key matching a substring of the prompt.
	for key, resp := range m.Responses {
		if key != "" && strings.Contains(req.Prompt, key) {
			return resp, nil
		}
	}
	// Fallback.
	if resp, ok := m.Responses[""]; ok {
		return resp, nil
	}
	return LLMResponse{Text: "mock response", PromptTokens: 10, CompletionTokens: 5}, nil
}
