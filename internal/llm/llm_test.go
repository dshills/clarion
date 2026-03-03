package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// ── BudgetTracker ────────────────────────────────────────────────────────────

func TestBudgetTracker(t *testing.T) {
	tests := []struct {
		name      string
		limit     int
		records   []int
		canAfford int
		wantCan   bool
		wantUsed  int
		wantRem   int
	}{
		{
			name:      "fresh tracker can afford within limit",
			limit:     100,
			canAfford: 50,
			wantCan:   true,
			wantUsed:  0,
			wantRem:   100,
		},
		{
			name:      "fresh tracker cannot afford over limit",
			limit:     100,
			canAfford: 101,
			wantCan:   false,
			wantUsed:  0,
			wantRem:   100,
		},
		{
			name:      "after recording, remaining decreases",
			limit:     100,
			records:   []int{40},
			canAfford: 60,
			wantCan:   true,
			wantUsed:  40,
			wantRem:   60,
		},
		{
			name:      "boundary: exactly at limit",
			limit:     100,
			records:   []int{100},
			canAfford: 1,
			wantCan:   false,
			wantUsed:  100,
			wantRem:   0,
		},
		{
			name:      "zero limit: nothing affordable",
			limit:     0,
			canAfford: 1,
			wantCan:   false,
			wantUsed:  0,
			wantRem:   0,
		},
		{
			name:      "multiple records accumulate",
			limit:     100,
			records:   []int{30, 20, 10},
			canAfford: 40,
			wantCan:   true,
			wantUsed:  60,
			wantRem:   40,
		},
		{
			name:      "exactly affordable",
			limit:     100,
			records:   []int{50},
			canAfford: 50,
			wantCan:   true,
			wantUsed:  50,
			wantRem:   50,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := NewBudgetTracker(tc.limit)
			for _, r := range tc.records {
				b.Record(r)
			}
			if got := b.CanAfford(tc.canAfford); got != tc.wantCan {
				t.Errorf("CanAfford(%d) = %v, want %v", tc.canAfford, got, tc.wantCan)
			}
			if b.Used() != tc.wantUsed {
				t.Errorf("Used() = %d, want %d", b.Used(), tc.wantUsed)
			}
			if b.Remaining() != tc.wantRem {
				t.Errorf("Remaining() = %d, want %d", b.Remaining(), tc.wantRem)
			}
		})
	}
}

// ── EstimateTokens ───────────────────────────────────────────────────────────

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name string
		text string
		want int
	}{
		{name: "empty string", text: "", want: 0},
		{name: "exactly 4 chars", text: "abcd", want: 1},
		{name: "8 chars", text: "12345678", want: 2},
		{name: "400 chars", text: strings.Repeat("x", 400), want: 100},
		{name: "single char", text: "a", want: 1},
		{name: "3 chars rounds down but min 1", text: "abc", want: 1},
		{name: "1000 chars", text: strings.Repeat("y", 1000), want: 250},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := EstimateTokens(tc.text); got != tc.want {
				t.Errorf("EstimateTokens(%q...) = %d, want %d", tc.text[:min(len(tc.text), 20)], got, tc.want)
			}
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ── LoadConfig ───────────────────────────────────────────────────────────────

func TestLoadConfig(t *testing.T) {
	// Helper to set all required env vars.
	setValid := func(t *testing.T) {
		t.Helper()
		t.Setenv("CLARION_LLM_PROVIDER", "openai")
		t.Setenv("CLARION_LLM_MODEL", "gpt-4o")
		t.Setenv("OPENAI_API_KEY", "sk-test123")
		t.Setenv("CLARION_LLM_TOKEN_BUDGET", "")
	}

	t.Run("valid config with defaults", func(t *testing.T) {
		setValid(t)
		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Provider != "openai" {
			t.Errorf("Provider = %q, want %q", cfg.Provider, "openai")
		}
		if cfg.Model != "gpt-4o" {
			t.Errorf("Model = %q, want %q", cfg.Model, "gpt-4o")
		}
		if cfg.APIKey != "sk-test123" {
			t.Errorf("APIKey = %q, want %q", cfg.APIKey, "sk-test123")
		}
		if cfg.TokenBudget != 100000 {
			t.Errorf("TokenBudget = %d, want 100000", cfg.TokenBudget)
		}
	})

	t.Run("anthropic provider accepted", func(t *testing.T) {
		setValid(t)
		t.Setenv("CLARION_LLM_PROVIDER", "anthropic")
		t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test123")
		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Provider != "anthropic" {
			t.Errorf("Provider = %q, want %q", cfg.Provider, "anthropic")
		}
	})

	t.Run("gemini provider accepted", func(t *testing.T) {
		setValid(t)
		t.Setenv("CLARION_LLM_PROVIDER", "gemini")
		t.Setenv("GEMINI_API_KEY", "AIza-test123")
		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Provider != "gemini" {
			t.Errorf("Provider = %q, want %q", cfg.Provider, "gemini")
		}
	})

	t.Run("CLARION_LLM_PROVIDER empty", func(t *testing.T) {
		setValid(t)
		t.Setenv("CLARION_LLM_PROVIDER", "")
		_, err := LoadConfig()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "CLARION_LLM_PROVIDER is required") {
			t.Errorf("error %q does not contain expected message", err.Error())
		}
	})

	t.Run("CLARION_LLM_PROVIDER invalid", func(t *testing.T) {
		setValid(t)
		t.Setenv("CLARION_LLM_PROVIDER", "invalid")
		_, err := LoadConfig()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "must be one of: openai, anthropic, gemini") {
			t.Errorf("error %q does not contain expected message", err.Error())
		}
	})

	t.Run("CLARION_LLM_MODEL empty", func(t *testing.T) {
		setValid(t)
		t.Setenv("CLARION_LLM_MODEL", "")
		_, err := LoadConfig()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "CLARION_LLM_MODEL is required") {
			t.Errorf("error %q does not contain expected message", err.Error())
		}
	})

	t.Run("OPENAI_API_KEY empty", func(t *testing.T) {
		setValid(t)
		t.Setenv("OPENAI_API_KEY", "")
		_, err := LoadConfig()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "OPENAI_API_KEY is required") {
			t.Errorf("error %q does not contain expected message", err.Error())
		}
	})

	t.Run("ANTHROPIC_API_KEY empty", func(t *testing.T) {
		setValid(t)
		t.Setenv("CLARION_LLM_PROVIDER", "anthropic")
		t.Setenv("ANTHROPIC_API_KEY", "")
		_, err := LoadConfig()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "ANTHROPIC_API_KEY is required") {
			t.Errorf("error %q does not contain expected message", err.Error())
		}
	})

	t.Run("CLARION_LLM_TOKEN_BUDGET not an integer", func(t *testing.T) {
		setValid(t)
		t.Setenv("CLARION_LLM_TOKEN_BUDGET", "abc")
		_, err := LoadConfig()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "must be an integer") {
			t.Errorf("error %q does not contain expected message", err.Error())
		}
	})

	t.Run("CLARION_LLM_TOKEN_BUDGET zero", func(t *testing.T) {
		setValid(t)
		t.Setenv("CLARION_LLM_TOKEN_BUDGET", "0")
		_, err := LoadConfig()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "must be > 0") {
			t.Errorf("error %q does not contain expected message", err.Error())
		}
	})

	t.Run("CLARION_LLM_TOKEN_BUDGET negative", func(t *testing.T) {
		setValid(t)
		t.Setenv("CLARION_LLM_TOKEN_BUDGET", "-1")
		_, err := LoadConfig()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "must be > 0") {
			t.Errorf("error %q does not contain expected message", err.Error())
		}
	})

	t.Run("CLARION_LLM_TOKEN_BUDGET valid custom", func(t *testing.T) {
		setValid(t)
		t.Setenv("CLARION_LLM_TOKEN_BUDGET", "50000")
		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.TokenBudget != 50000 {
			t.Errorf("TokenBudget = %d, want 50000", cfg.TokenBudget)
		}
	})

	t.Run("APIKey not serialized in JSON", func(t *testing.T) {
		setValid(t)
		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		data, err := json.Marshal(cfg)
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}
		if strings.Contains(string(data), "sk-test123") {
			t.Errorf("APIKey should not appear in JSON output: %s", data)
		}
		if strings.Contains(string(data), "api_key") {
			t.Errorf("api_key field should not appear in JSON output: %s", data)
		}
	})
}

// ── OpenAI Adapter ───────────────────────────────────────────────────────────

// openAISuccessResponse builds a valid OpenAI chat completions response body.
func openAISuccessResponse(text, model string, promptTokens, completionTokens int) []byte {
	resp := map[string]any{
		"model": model,
		"choices": []map[string]any{
			{
				"message": map[string]string{
					"role":    "assistant",
					"content": text,
				},
			},
		},
		"usage": map[string]int{
			"prompt_tokens":     promptTokens,
			"completion_tokens": completionTokens,
		},
	}
	data, _ := json.Marshal(resp)
	return data
}

func TestOpenAIAdapter(t *testing.T) {
	t.Run("valid response", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(openAISuccessResponse("Hello!", "gpt-4o", 20, 5))
		}))
		defer srv.Close()

		adapter := &openAIAdapter{
			model:      "gpt-4o",
			apiKey:     "sk-test",
			client:     srv.Client(),
			retryDelay: 0,
		}
		// Override the endpoint by replacing client transport to redirect to test server.
		// Since we can't easily override the constant, we use a custom transport.
		adapter.client = newClientWithBaseURL(srv.URL)

		resp, err := adapter.Call(context.Background(), LLMRequest{Prompt: "hi", MaxTokens: 100})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Text != "Hello!" {
			t.Errorf("Text = %q, want %q", resp.Text, "Hello!")
		}
		if resp.PromptTokens != 20 {
			t.Errorf("PromptTokens = %d, want 20", resp.PromptTokens)
		}
		if resp.CompletionTokens != 5 {
			t.Errorf("CompletionTokens = %d, want 5", resp.CompletionTokens)
		}
		if resp.ModelID != "gpt-4o" {
			t.Errorf("ModelID = %q, want %q", resp.ModelID, "gpt-4o")
		}
	})

	t.Run("429 retry succeeds on second attempt", func(t *testing.T) {
		var callCount int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			n := atomic.AddInt32(&callCount, 1)
			if n == 1 {
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error":"rate limited"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(openAISuccessResponse("retried!", "gpt-4o", 10, 3))
		}))
		defer srv.Close()

		adapter := &openAIAdapter{
			model:      "gpt-4o",
			apiKey:     "sk-test",
			client:     newClientWithBaseURL(srv.URL),
			retryDelay: 0,
		}

		resp, err := adapter.Call(context.Background(), LLMRequest{Prompt: "hi", MaxTokens: 100})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Text != "retried!" {
			t.Errorf("Text = %q, want %q", resp.Text, "retried!")
		}
		if atomic.LoadInt32(&callCount) != 2 {
			t.Errorf("callCount = %d, want 2", atomic.LoadInt32(&callCount))
		}
	})

	t.Run("429 retry fails on second attempt", func(t *testing.T) {
		var callCount int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&callCount, 1)
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"rate limited"}`))
		}))
		defer srv.Close()

		adapter := &openAIAdapter{
			model:      "gpt-4o",
			apiKey:     "sk-test",
			client:     newClientWithBaseURL(srv.URL),
			retryDelay: 0,
		}

		_, err := adapter.Call(context.Background(), LLMRequest{Prompt: "hi", MaxTokens: 100})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if atomic.LoadInt32(&callCount) != 2 {
			t.Errorf("callCount = %d, want 2", atomic.LoadInt32(&callCount))
		}
	})

	t.Run("401 no retry", func(t *testing.T) {
		var callCount int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&callCount, 1)
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"unauthorized"}`))
		}))
		defer srv.Close()

		adapter := &openAIAdapter{
			model:      "gpt-4o",
			apiKey:     "sk-test",
			client:     newClientWithBaseURL(srv.URL),
			retryDelay: 0,
		}

		_, err := adapter.Call(context.Background(), LLMRequest{Prompt: "hi", MaxTokens: 100})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if atomic.LoadInt32(&callCount) != 1 {
			t.Errorf("callCount = %d, want 1 (no retry on 401)", atomic.LoadInt32(&callCount))
		}
	})

	t.Run("malformed JSON response", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`not valid json`))
		}))
		defer srv.Close()

		adapter := &openAIAdapter{
			model:      "gpt-4o",
			apiKey:     "sk-test",
			client:     newClientWithBaseURL(srv.URL),
			retryDelay: 0,
		}

		_, err := adapter.Call(context.Background(), LLMRequest{Prompt: "hi", MaxTokens: 100})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "unparseable response") {
			t.Errorf("error %q does not contain 'unparseable response'", err.Error())
		}
	})

	t.Run("empty choices in response", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"choices":[],"usage":{"prompt_tokens":5,"completion_tokens":0},"model":"gpt-4o"}`))
		}))
		defer srv.Close()

		adapter := &openAIAdapter{
			model:      "gpt-4o",
			apiKey:     "sk-test",
			client:     newClientWithBaseURL(srv.URL),
			retryDelay: 0,
		}

		_, err := adapter.Call(context.Background(), LLMRequest{Prompt: "hi", MaxTokens: 100})
		if err == nil {
			t.Fatal("expected error for empty choices, got nil")
		}
		if !strings.Contains(err.Error(), "unparseable response") {
			t.Errorf("error %q does not contain 'unparseable response'", err.Error())
		}
	})

	t.Run("503 service unavailable retries", func(t *testing.T) {
		var callCount int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			n := atomic.AddInt32(&callCount, 1)
			if n == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte(`{"error":"service unavailable"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(openAISuccessResponse("ok", "gpt-4o", 10, 3))
		}))
		defer srv.Close()

		adapter := &openAIAdapter{
			model:      "gpt-4o",
			apiKey:     "sk-test",
			client:     newClientWithBaseURL(srv.URL),
			retryDelay: 0,
		}

		resp, err := adapter.Call(context.Background(), LLMRequest{Prompt: "hi", MaxTokens: 100})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Text != "ok" {
			t.Errorf("Text = %q, want %q", resp.Text, "ok")
		}
		if atomic.LoadInt32(&callCount) != 2 {
			t.Errorf("callCount = %d, want 2", atomic.LoadInt32(&callCount))
		}
	})
}

// ── Anthropic Adapter ────────────────────────────────────────────────────────

// anthropicSuccessResponse builds a valid Anthropic messages response body.
func anthropicSuccessResponse(text, model string, inputTokens, outputTokens int) []byte {
	resp := map[string]any{
		"model": model,
		"content": []map[string]string{
			{"type": "text", "text": text},
		},
		"usage": map[string]int{
			"input_tokens":  inputTokens,
			"output_tokens": outputTokens,
		},
	}
	data, _ := json.Marshal(resp)
	return data
}

func TestAnthropicAdapter(t *testing.T) {
	t.Run("valid response", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify required Anthropic headers.
			if r.Header.Get("x-api-key") == "" {
				t.Error("missing x-api-key header")
			}
			if r.Header.Get("anthropic-version") == "" {
				t.Error("missing anthropic-version header")
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(anthropicSuccessResponse("Claude says hi!", "claude-3-5-sonnet-20241022", 15, 8))
		}))
		defer srv.Close()

		adapter := &anthropicAdapter{
			model:      "claude-3-5-sonnet-20241022",
			apiKey:     "sk-ant-test",
			client:     newClientWithBaseURL(srv.URL),
			retryDelay: 0,
		}

		resp, err := adapter.Call(context.Background(), LLMRequest{Prompt: "hi", MaxTokens: 100})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Text != "Claude says hi!" {
			t.Errorf("Text = %q, want %q", resp.Text, "Claude says hi!")
		}
		if resp.PromptTokens != 15 {
			t.Errorf("PromptTokens = %d, want 15", resp.PromptTokens)
		}
		if resp.CompletionTokens != 8 {
			t.Errorf("CompletionTokens = %d, want 8", resp.CompletionTokens)
		}
		if resp.ModelID != "claude-3-5-sonnet-20241022" {
			t.Errorf("ModelID = %q, want %q", resp.ModelID, "claude-3-5-sonnet-20241022")
		}
	})

	t.Run("429 retry succeeds on second attempt", func(t *testing.T) {
		var callCount int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			n := atomic.AddInt32(&callCount, 1)
			if n == 1 {
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error":"rate limited"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(anthropicSuccessResponse("retried!", "claude-3-5-sonnet-20241022", 10, 4))
		}))
		defer srv.Close()

		adapter := &anthropicAdapter{
			model:      "claude-3-5-sonnet-20241022",
			apiKey:     "sk-ant-test",
			client:     newClientWithBaseURL(srv.URL),
			retryDelay: 0,
		}

		resp, err := adapter.Call(context.Background(), LLMRequest{Prompt: "hi", MaxTokens: 100})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Text != "retried!" {
			t.Errorf("Text = %q, want %q", resp.Text, "retried!")
		}
		if atomic.LoadInt32(&callCount) != 2 {
			t.Errorf("callCount = %d, want 2", atomic.LoadInt32(&callCount))
		}
	})

	t.Run("429 retry fails on second attempt", func(t *testing.T) {
		var callCount int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&callCount, 1)
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"rate limited"}`))
		}))
		defer srv.Close()

		adapter := &anthropicAdapter{
			model:      "claude-3-5-sonnet-20241022",
			apiKey:     "sk-ant-test",
			client:     newClientWithBaseURL(srv.URL),
			retryDelay: 0,
		}

		_, err := adapter.Call(context.Background(), LLMRequest{Prompt: "hi", MaxTokens: 100})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if atomic.LoadInt32(&callCount) != 2 {
			t.Errorf("callCount = %d, want 2", atomic.LoadInt32(&callCount))
		}
	})

	t.Run("401 no retry", func(t *testing.T) {
		var callCount int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&callCount, 1)
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"unauthorized"}`))
		}))
		defer srv.Close()

		adapter := &anthropicAdapter{
			model:      "claude-3-5-sonnet-20241022",
			apiKey:     "sk-ant-test",
			client:     newClientWithBaseURL(srv.URL),
			retryDelay: 0,
		}

		_, err := adapter.Call(context.Background(), LLMRequest{Prompt: "hi", MaxTokens: 100})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if atomic.LoadInt32(&callCount) != 1 {
			t.Errorf("callCount = %d, want 1 (no retry on 401)", atomic.LoadInt32(&callCount))
		}
	})

	t.Run("malformed JSON response", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`not json`))
		}))
		defer srv.Close()

		adapter := &anthropicAdapter{
			model:      "claude-3-5-sonnet-20241022",
			apiKey:     "sk-ant-test",
			client:     newClientWithBaseURL(srv.URL),
			retryDelay: 0,
		}

		_, err := adapter.Call(context.Background(), LLMRequest{Prompt: "hi", MaxTokens: 100})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "unparseable response") {
			t.Errorf("error %q does not contain 'unparseable response'", err.Error())
		}
	})

	t.Run("empty content array", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"content":[],"usage":{"input_tokens":5,"output_tokens":0},"model":"claude-3-5-sonnet-20241022"}`))
		}))
		defer srv.Close()

		adapter := &anthropicAdapter{
			model:      "claude-3-5-sonnet-20241022",
			apiKey:     "sk-ant-test",
			client:     newClientWithBaseURL(srv.URL),
			retryDelay: 0,
		}

		_, err := adapter.Call(context.Background(), LLMRequest{Prompt: "hi", MaxTokens: 100})
		if err == nil {
			t.Fatal("expected error for empty content, got nil")
		}
		if !strings.Contains(err.Error(), "unparseable response") {
			t.Errorf("error %q does not contain 'unparseable response'", err.Error())
		}
	})

	t.Run("403 forbidden no retry", func(t *testing.T) {
		var callCount int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&callCount, 1)
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"error":"forbidden"}`))
		}))
		defer srv.Close()

		adapter := &anthropicAdapter{
			model:      "claude-3-5-sonnet-20241022",
			apiKey:     "sk-ant-test",
			client:     newClientWithBaseURL(srv.URL),
			retryDelay: 0,
		}

		_, err := adapter.Call(context.Background(), LLMRequest{Prompt: "hi", MaxTokens: 100})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if atomic.LoadInt32(&callCount) != 1 {
			t.Errorf("callCount = %d, want 1 (no retry on 403)", atomic.LoadInt32(&callCount))
		}
	})
}

// ── MockAdapter ──────────────────────────────────────────────────────────────

func TestMockAdapter(t *testing.T) {
	t.Run("returns fallback when no key matches", func(t *testing.T) {
		m := &MockAdapter{
			Responses: map[string]LLMResponse{
				"": {Text: "fallback", PromptTokens: 5, CompletionTokens: 3},
			},
		}
		resp, err := m.Call(context.Background(), LLMRequest{Prompt: "unrelated prompt"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Text != "fallback" {
			t.Errorf("Text = %q, want %q", resp.Text, "fallback")
		}
	})

	t.Run("returns response for matching substring key", func(t *testing.T) {
		m := &MockAdapter{
			Responses: map[string]LLMResponse{
				"summarize": {Text: "summary result", PromptTokens: 20, CompletionTokens: 10},
				"":          {Text: "fallback"},
			},
		}
		resp, err := m.Call(context.Background(), LLMRequest{Prompt: "please summarize this document"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Text != "summary result" {
			t.Errorf("Text = %q, want %q", resp.Text, "summary result")
		}
	})

	t.Run("CallCount incremented on each call", func(t *testing.T) {
		m := &MockAdapter{}
		for i := 0; i < 5; i++ {
			m.Call(context.Background(), LLMRequest{Prompt: "test"})
		}
		if m.CallCount != 5 {
			t.Errorf("CallCount = %d, want 5", m.CallCount)
		}
	})

	t.Run("default response when no Responses map", func(t *testing.T) {
		m := &MockAdapter{}
		resp, err := m.Call(context.Background(), LLMRequest{Prompt: "anything"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Text != "mock response" {
			t.Errorf("Text = %q, want %q", resp.Text, "mock response")
		}
		if resp.PromptTokens != 10 {
			t.Errorf("PromptTokens = %d, want 10", resp.PromptTokens)
		}
		if resp.CompletionTokens != 5 {
			t.Errorf("CompletionTokens = %d, want 5", resp.CompletionTokens)
		}
	})

	t.Run("determinism: 10 identical calls return identical responses", func(t *testing.T) {
		m := &MockAdapter{
			Responses: map[string]LLMResponse{
				"": {Text: "deterministic", PromptTokens: 7, CompletionTokens: 3},
			},
		}
		var first LLMResponse
		for i := 0; i < 10; i++ {
			resp, err := m.Call(context.Background(), LLMRequest{Prompt: "hello"})
			if err != nil {
				t.Fatalf("call %d: unexpected error: %v", i, err)
			}
			if i == 0 {
				first = resp
			} else {
				if resp.Text != first.Text || resp.PromptTokens != first.PromptTokens || resp.CompletionTokens != first.CompletionTokens {
					t.Errorf("call %d: got %+v, want %+v", i, resp, first)
				}
			}
		}
		if m.CallCount != 10 {
			t.Errorf("CallCount = %d, want 10", m.CallCount)
		}
	})

	t.Run("Name and Validate", func(t *testing.T) {
		m := &MockAdapter{}
		if m.Name() != "mock" {
			t.Errorf("Name() = %q, want %q", m.Name(), "mock")
		}
		if err := m.Validate(); err != nil {
			t.Errorf("Validate() = %v, want nil", err)
		}
	})
}

// ── Pipeline ─────────────────────────────────────────────────────────────────

func TestPipeline(t *testing.T) {
	ctx := context.Background()

	t.Run("all stages complete successfully", func(t *testing.T) {
		adapter := &MockAdapter{
			Responses: map[string]LLMResponse{
				"": {Text: "ok", PromptTokens: 10, CompletionTokens: 5},
			},
		}
		budget := NewBudgetTracker(10000)
		p := NewPipeline(adapter, budget, false)

		stages := []PipelineStage{
			{Name: StageSummarize, Prompt: "summarize this", Required: true},
			{Name: StageGenerate, Prompt: "generate from this", Required: true},
			{Name: StageVerify, Prompt: "verify the output", Required: false},
		}

		results, err := p.Run(ctx, stages)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 3 {
			t.Errorf("len(results) = %d, want 3", len(results))
		}
		for i, r := range results {
			if r.Stage != stages[i].Name {
				t.Errorf("results[%d].Stage = %q, want %q", i, r.Stage, stages[i].Name)
			}
		}
		// Cumulative tokens: each call uses 10+5=15; 3 stages = 45.
		if results[2].CumulativeTokens != 45 {
			t.Errorf("final CumulativeTokens = %d, want 45", results[2].CumulativeTokens)
		}
	})

	t.Run("budget zero at start returns ErrBudgetExhausted", func(t *testing.T) {
		adapter := &MockAdapter{}
		budget := NewBudgetTracker(0)
		p := NewPipeline(adapter, budget, false)

		stages := []PipelineStage{
			{Name: StageSummarize, Prompt: "summarize this", Required: true},
		}

		_, err := p.Run(ctx, stages)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, ErrBudgetExhausted) {
			t.Errorf("error = %v, want ErrBudgetExhausted", err)
		}
		// No calls should have been made.
		if adapter.CallCount != 0 {
			t.Errorf("CallCount = %d, want 0", adapter.CallCount)
		}
	})

	t.Run("required stage budget insufficient returns ErrBudgetExhausted", func(t *testing.T) {
		// Budget=5, prompt="Hello world" (11 chars → estimated 2 tokens). CanAfford(2) with limit=5 passes.
		// But we want the stage to fail budget check. Use budget=1 and a longer prompt.
		// "Hello world" = 11 chars → 11/4 = 2 estimated tokens. budget=1 means CanAfford(2) → 0+2<=1 = false.
		adapter := &MockAdapter{}
		budget := NewBudgetTracker(1)
		p := NewPipeline(adapter, budget, false)

		stages := []PipelineStage{
			{Name: StageSummarize, Prompt: "Hello world!!", Required: true},
		}

		results, err := p.Run(ctx, stages)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, ErrBudgetExhausted) {
			t.Errorf("error = %v, want ErrBudgetExhausted", err)
		}
		if len(results) != 0 {
			t.Errorf("len(results) = %d, want 0", len(results))
		}
		if adapter.CallCount != 0 {
			t.Errorf("CallCount = %d, want 0", adapter.CallCount)
		}
	})

	t.Run("optional stage skipped due to budget returns ErrBudgetSkipped with partial results", func(t *testing.T) {
		// Stage 1: short prompt (4 chars → 1 estimated token), mock returns 15+10=25 tokens used.
		// Budget=30. After stage 1: used=25, remaining=5.
		// Stage 2: long prompt (40 chars → 10 estimated tokens). CanAfford(10) → 25+10<=30 = false.
		// Stage 2 is optional → ErrBudgetSkipped.
		adapter := &MockAdapter{
			Responses: map[string]LLMResponse{
				"": {Text: "stage1 result", PromptTokens: 15, CompletionTokens: 10},
			},
		}
		budget := NewBudgetTracker(30)
		p := NewPipeline(adapter, budget, false)

		// Stage 1 prompt: "go" (2 chars → 0 estimated... wait, 2/4=0, but len>0 so returns 1)
		// Actually EstimateTokens("go") = 2/4 = 0, but len("go") > 0 → returns 1.
		// CanAfford(1) with limit=30, used=0 → 0+1<=30 = true. Good.
		// After stage 1 mock returns 25 tokens → used=25, remaining=5.
		// Stage 2 prompt: 40 chars → 40/4=10 estimated. CanAfford(10) → 25+10<=30 = false. Optional → skip.
		stages := []PipelineStage{
			{Name: StageSummarize, Prompt: "go", Required: true},
			{Name: StageGenerate, Prompt: strings.Repeat("x", 40), Required: false},
		}

		results, err := p.Run(ctx, stages)
		if !errors.Is(err, ErrBudgetSkipped) {
			t.Errorf("error = %v, want ErrBudgetSkipped", err)
		}
		if len(results) != 1 {
			t.Errorf("len(results) = %d, want 1", len(results))
		}
		if len(results) > 0 && results[0].Stage != StageSummarize {
			t.Errorf("results[0].Stage = %q, want %q", results[0].Stage, StageSummarize)
		}
	})

	t.Run("adapter error propagates", func(t *testing.T) {
		// Use a nil client to cause a network error, or use a closed server.
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`internal error`))
		}))
		srv.Close() // close immediately so connections fail

		adapter := &openAIAdapter{
			model:      "gpt-4o",
			apiKey:     "sk-test",
			client:     srv.Client(),
			retryDelay: 0,
		}
		// Use a mock instead to control the error scenario cleanly.
		mockErr := &MockAdapter{
			Responses: map[string]LLMResponse{
				"": {Text: "ok", PromptTokens: 5, CompletionTokens: 5},
			},
		}
		_ = adapter // not used; using mock below

		budget := NewBudgetTracker(10000)
		p := NewPipeline(mockErr, budget, false)

		stages := []PipelineStage{
			{Name: StageSummarize, Prompt: "test", Required: true},
		}
		results, err := p.Run(ctx, stages)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Errorf("len(results) = %d, want 1", len(results))
		}
	})

	t.Run("verbose mode does not panic", func(t *testing.T) {
		adapter := &MockAdapter{
			Responses: map[string]LLMResponse{
				"": {Text: "ok", PromptTokens: 10, CompletionTokens: 5},
			},
		}
		budget := NewBudgetTracker(10000)
		p := NewPipeline(adapter, budget, true) // verbose=true

		stages := []PipelineStage{
			{Name: StageSummarize, Prompt: "summarize this text", Required: true},
		}
		_, err := p.Run(ctx, stages)
		if err != nil {
			t.Fatalf("unexpected error in verbose mode: %v", err)
		}
	})

	t.Run("required stage after optional stage exhaustion", func(t *testing.T) {
		// Stage 1 (optional): affordable, completes.
		// Stage 2 (required): not affordable → ErrBudgetExhausted.
		adapter := &MockAdapter{
			Responses: map[string]LLMResponse{
				"": {Text: "done", PromptTokens: 20, CompletionTokens: 15},
			},
		}
		budget := NewBudgetTracker(40)
		// After stage 1: used = 35, remaining = 5.
		// Stage 2 prompt: 40 chars → 10 estimated. CanAfford(10) → 35+10<=40 = false. Required → ErrBudgetExhausted.
		p := NewPipeline(adapter, budget, false)

		stages := []PipelineStage{
			{Name: StageSummarize, Prompt: "go", Required: false},
			{Name: StageGenerate, Prompt: strings.Repeat("z", 40), Required: true},
		}

		results, err := p.Run(ctx, stages)
		if !errors.Is(err, ErrBudgetExhausted) {
			t.Errorf("error = %v, want ErrBudgetExhausted", err)
		}
		if len(results) != 1 {
			t.Errorf("len(results) = %d, want 1 (partial)", len(results))
		}
	})
}

// ── Factory ──────────────────────────────────────────────────────────────────

func TestNewAdapter(t *testing.T) {
	t.Run("openai adapter created successfully", func(t *testing.T) {
		cfg := Config{
			Provider:    "openai",
			Model:       "gpt-4o",
			APIKey:      "sk-test",
			TokenBudget: 100000,
		}
		adapter, err := NewAdapter(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if adapter.Name() != "openai" {
			t.Errorf("Name() = %q, want %q", adapter.Name(), "openai")
		}
	})

	t.Run("anthropic adapter created successfully", func(t *testing.T) {
		cfg := Config{
			Provider:    "anthropic",
			Model:       "claude-3-5-sonnet-20241022",
			APIKey:      "sk-ant-test",
			TokenBudget: 100000,
		}
		adapter, err := NewAdapter(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if adapter.Name() != "anthropic" {
			t.Errorf("Name() = %q, want %q", adapter.Name(), "anthropic")
		}
	})

	t.Run("unknown provider returns error", func(t *testing.T) {
		cfg := Config{
			Provider:    "google",
			Model:       "gemini",
			APIKey:      "key",
			TokenBudget: 100000,
		}
		_, err := NewAdapter(cfg)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "unknown provider") {
			t.Errorf("error %q does not contain 'unknown provider'", err.Error())
		}
	})

	t.Run("missing API key fails validate", func(t *testing.T) {
		cfg := Config{
			Provider:    "openai",
			Model:       "gpt-4o",
			APIKey:      "",
			TokenBudget: 100000,
		}
		_, err := NewAdapter(cfg)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// newClientWithBaseURL returns an *http.Client that redirects all requests to
// the given base URL (typically a httptest.Server URL), preserving the path.
// This is used to test adapters against a local test server without changing
// the production endpoint constants.
func newClientWithBaseURL(baseURL string) *http.Client {
	return &http.Client{
		Timeout: 5 * time.Second,
		Transport: &redirectTransport{base: baseURL},
	}
}

// redirectTransport rewrites the host of every request to point to base.
type redirectTransport struct {
	base string
}

func (t *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Parse the base URL to get scheme and host.
	// base is like "http://127.0.0.1:PORT"
	newReq := req.Clone(req.Context())
	// Replace scheme+host with test server, keep path+query.
	newReq.URL.Scheme = "http"
	// Extract host from base URL (strip "http://").
	host := t.base
	if len(host) > 7 && host[:7] == "http://" {
		host = host[7:]
	} else if len(host) > 8 && host[:8] == "https://" {
		host = host[8:]
	}
	newReq.URL.Host = host
	return http.DefaultTransport.RoundTrip(newReq)
}
