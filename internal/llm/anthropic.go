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

const (
	anthropicEndpoint = "https://api.anthropic.com/v1/messages"
	anthropicVersion  = "2023-06-01"
)

// anthropicAdapter calls the Anthropic messages API.
type anthropicAdapter struct {
	model      string
	apiKey     string
	client     *http.Client
	retryDelay time.Duration // retry delay between attempts; set by factory
}

func (a *anthropicAdapter) Name() string { return "anthropic" }

func (a *anthropicAdapter) Validate() error {
	if a.apiKey == "" {
		return fmt.Errorf("anthropic: API key is empty")
	}
	if a.model == "" {
		return fmt.Errorf("anthropic: model is empty")
	}
	return nil
}

func (a *anthropicAdapter) Call(ctx context.Context, req LLMRequest) (LLMResponse, error) {
	return withRetry(func() (LLMResponse, int, error) {
		return a.call(ctx, req)
	}, a.retryDelay)
}

func (a *anthropicAdapter) call(ctx context.Context, req LLMRequest) (LLMResponse, int, error) {
	body, err := json.Marshal(map[string]any{
		"model":       a.model,
		"temperature": 0,
		"max_tokens":  req.MaxTokens,
		"messages": []map[string]string{
			{"role": "user", "content": req.Prompt},
		},
	})
	if err != nil {
		return LLMResponse{}, 0, fmt.Errorf("anthropic: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicEndpoint, bytes.NewReader(body))
	if err != nil {
		return LLMResponse{}, 0, fmt.Errorf("anthropic: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", a.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)

	start := time.Now()
	httpResp, err := a.client.Do(httpReq)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return LLMResponse{}, 0, networkError("anthropic", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return LLMResponse{}, httpResp.StatusCode, fmt.Errorf("anthropic: read response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return LLMResponse{}, httpResp.StatusCode, httpError("anthropic", a.model, httpResp.StatusCode, respBody)
	}

	var parsed struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
		Model string `json:"model"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil || len(parsed.Content) == 0 {
		return LLMResponse{}, http.StatusOK, parseError("anthropic", a.model, respBody)
	}

	return LLMResponse{
		Text:             parsed.Content[0].Text,
		PromptTokens:     parsed.Usage.InputTokens,
		CompletionTokens: parsed.Usage.OutputTokens,
		ModelID:          parsed.Model,
		LatencyMS:        latency,
	}, http.StatusOK, nil
}
