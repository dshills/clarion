package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const openAIEndpoint = "https://api.openai.com/v1/chat/completions"

// openAIAdapter calls the OpenAI chat completions API.
type openAIAdapter struct {
	model      string
	apiKey     string
	client     *http.Client
	retryDelay time.Duration // retry delay between attempts; set by factory
}

func (a *openAIAdapter) Name() string { return "openai" }

func (a *openAIAdapter) Validate() error {
	if a.apiKey == "" {
		return fmt.Errorf("openai: API key is empty")
	}
	if a.model == "" {
		return fmt.Errorf("openai: model is empty")
	}
	return nil
}

func (a *openAIAdapter) Call(ctx context.Context, req LLMRequest) (LLMResponse, error) {
	return withRetry(func() (LLMResponse, int, error) {
		return a.call(ctx, req)
	}, a.retryDelay)
}

func (a *openAIAdapter) call(ctx context.Context, req LLMRequest) (LLMResponse, int, error) {
	body, err := json.Marshal(map[string]any{
		"model":       a.model,
		"temperature": 0,
		"max_tokens":  req.MaxTokens,
		"messages": []map[string]string{
			{"role": "user", "content": req.Prompt},
		},
	})
	if err != nil {
		return LLMResponse{}, 0, fmt.Errorf("openai: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIEndpoint, bytes.NewReader(body))
	if err != nil {
		return LLMResponse{}, 0, fmt.Errorf("openai: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+a.apiKey)

	start := time.Now()
	httpResp, err := a.client.Do(httpReq)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return LLMResponse{}, 0, networkError("openai", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return LLMResponse{}, httpResp.StatusCode, fmt.Errorf("openai: read response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return LLMResponse{}, httpResp.StatusCode, httpError("openai", a.model, httpResp.StatusCode, respBody)
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
		Model string `json:"model"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil || len(parsed.Choices) == 0 {
		return LLMResponse{}, http.StatusOK, parseError("openai", a.model, respBody)
	}

	return LLMResponse{
		Text:             parsed.Choices[0].Message.Content,
		PromptTokens:     parsed.Usage.PromptTokens,
		CompletionTokens: parsed.Usage.CompletionTokens,
		ModelID:          parsed.Model,
		LatencyMS:        latency,
	}, http.StatusOK, nil
}
