package llm_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"os"
	"testing"

	. "github.com/clarion-dev/clarion/internal/llm"
)

const fakeAPIKey = "sk-FAKE-KEY-clarion-test-do-not-use-XYZ987"

// TestAPIKeyNotLogged verifies that the provider API key never appears in any
// log output, error message, or JSON-serialized Config.
func TestAPIKeyNotLogged(t *testing.T) {
	t.Run("ConfigJSONOmitsKey", func(t *testing.T) {
		t.Setenv("CLARION_LLM_PROVIDER", "openai")
		t.Setenv("CLARION_LLM_MODEL", "gpt-4o")
		t.Setenv("OPENAI_API_KEY", fakeAPIKey)

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}

		data, err := json.Marshal(cfg)
		if err != nil {
			t.Fatalf("json.Marshal: %v", err)
		}
		if bytes.Contains(data, []byte(fakeAPIKey)) {
			t.Errorf("API key found in JSON-serialized Config; Config.APIKey must be tagged json:\"-\"")
		}
	})

	t.Run("VerbosePipelineDoesNotLogKey", func(t *testing.T) {
		// Redirect the default logger to a buffer for the duration of this test.
		var logBuf bytes.Buffer
		log.SetOutput(&logBuf)
		defer log.SetOutput(os.Stderr)

		adapter := &MockAdapter{
			Responses: map[string]LLMResponse{
				"": {Text: "response", PromptTokens: 10, CompletionTokens: 5},
			},
		}
		budget := NewBudgetTracker(100000)
		// verbose=true triggers log.Printf calls inside Pipeline.Run.
		pipeline := NewPipeline(adapter, budget, true)

		stages := []PipelineStage{
			{Name: StageGenerate, Prompt: "some prompt " + fakeAPIKey, Required: true},
		}
		if _, err := pipeline.Run(context.Background(), stages); err != nil {
			t.Fatalf("pipeline.Run: %v", err)
		}

		if bytes.Contains(logBuf.Bytes(), []byte(fakeAPIKey)) {
			t.Errorf("API key found in verbose pipeline log output:\n%s", logBuf.String())
		}
	})
}
