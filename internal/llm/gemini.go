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

const geminiEndpoint = "https://generativelanguage.googleapis.com/v1beta/models"

// geminiAdapter calls the Google Gemini generateContent API.
type geminiAdapter struct {
	model      string
	apiKey     string
	client     *http.Client
	retryDelay time.Duration // retry delay between attempts; set by factory
}

func (a *geminiAdapter) Name() string { return "gemini" }

func (a *geminiAdapter) Validate() error {
	if a.apiKey == "" {
		return fmt.Errorf("gemini: API key is empty")
	}
	if a.model == "" {
		return fmt.Errorf("gemini: model is empty")
	}
	return nil
}

func (a *geminiAdapter) Call(ctx context.Context, req LLMRequest) (LLMResponse, error) {
	return withRetry(func() (LLMResponse, int, error) {
		return a.call(ctx, req)
	}, a.retryDelay)
}

func (a *geminiAdapter) call(ctx context.Context, req LLMRequest) (LLMResponse, int, error) {
	body, err := json.Marshal(map[string]any{
		"contents": []map[string]any{
			{
				"parts": []map[string]string{
					{"text": req.Prompt},
				},
			},
		},
		"generationConfig": map[string]any{
			"temperature":     0,
			"maxOutputTokens": req.MaxTokens,
		},
	})
	if err != nil {
		return LLMResponse{}, 0, fmt.Errorf("gemini: marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/%s:generateContent?key=%s", geminiEndpoint, a.model, a.apiKey)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return LLMResponse{}, 0, fmt.Errorf("gemini: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	start := time.Now()
	httpResp, err := a.client.Do(httpReq)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return LLMResponse{}, 0, networkError("gemini", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return LLMResponse{}, httpResp.StatusCode, fmt.Errorf("gemini: read response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return LLMResponse{}, httpResp.StatusCode, httpError("gemini", a.model, httpResp.StatusCode, respBody)
	}

	var parsed struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
		} `json:"usageMetadata"`
		ModelVersion string `json:"modelVersion"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil ||
		len(parsed.Candidates) == 0 ||
		len(parsed.Candidates[0].Content.Parts) == 0 {
		return LLMResponse{}, http.StatusOK, parseError("gemini", a.model, respBody)
	}

	return LLMResponse{
		Text:             parsed.Candidates[0].Content.Parts[0].Text,
		PromptTokens:     parsed.UsageMetadata.PromptTokenCount,
		CompletionTokens: parsed.UsageMetadata.CandidatesTokenCount,
		ModelID:          a.model,
		LatencyMS:        latency,
	}, http.StatusOK, nil
}
