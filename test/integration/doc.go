// Package integration contains end-to-end tests for Clarion that require
// real LLM API keys and make network calls. Tests are gated by the
// "integration" build tag so they are excluded from normal `go test ./...` runs.
//
// Run with:
//
//	go test -tags integration -timeout 300s ./test/integration/
//
// Required environment variables (see README):
//
//	CLARION_LLM_PROVIDER   openai | anthropic
//	CLARION_LLM_MODEL      model name matching the provider
//	OPENAI_API_KEY         if using openai
//	ANTHROPIC_API_KEY      if using anthropic
//
// Per-provider smoke tests (TestLLMProviderOpenAI, TestLLMProviderAnthropic)
// skip automatically when their respective API keys are absent.
//
// TestPackEnterprise and TestDrift* do not require LLM keys; the drift and
// scanner tests are pure Go. TestPackEnterprise does require a configured
// provider and will skip if none is available.
package integration
