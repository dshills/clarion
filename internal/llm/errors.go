package llm

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// Sentinel errors returned by the pipeline.
var (
	// ErrBudgetExhausted is returned when a required stage cannot be afforded.
	ErrBudgetExhausted = errors.New("token budget exhausted before required stage")

	// ErrBudgetSkipped is returned when an optional stage is skipped due to budget.
	ErrBudgetSkipped = errors.New("token budget exceeded: optional stage skipped")
)

// isRetryable returns true for HTTP status codes that warrant a single retry.
func isRetryable(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests ||
		statusCode == http.StatusServiceUnavailable ||
		statusCode == http.StatusGatewayTimeout
}

// isFatal returns true for HTTP status codes where retrying would not help.
func isFatal(statusCode int) bool {
	return statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden
}

// withRetry calls fn once. If it returns a retryable HTTP status code, it
// waits retryDelay then calls fn a second time. On second failure, the error
// from the second attempt is returned.
func withRetry(fn func() (LLMResponse, int, error), retryDelay time.Duration) (LLMResponse, error) {
	resp, status, err := fn()
	if err == nil {
		return resp, nil
	}

	// Do not retry context cancellation or fatal auth errors.
	if errors.Is(err, context.DeadlineExceeded) || isFatal(status) {
		return LLMResponse{}, err
	}

	if isRetryable(status) {
		time.Sleep(retryDelay)
		resp2, _, err2 := fn()
		return resp2, err2
	}

	return LLMResponse{}, err
}

// httpError constructs an error from an unexpected HTTP status.
func httpError(provider, model string, statusCode int, body []byte) error {
	preview := string(body)
	if len(preview) > 200 {
		preview = preview[:200]
	}
	return fmt.Errorf("provider %s: HTTP %d (model=%s): %s", provider, statusCode, model, preview)
}

// parseError constructs an error for an unparseable response body.
func parseError(provider, model string, body []byte) error {
	preview := string(body)
	if len(preview) > 200 {
		preview = preview[:200]
	}
	return fmt.Errorf("provider %s: unparseable response (model=%s): %s", provider, model, preview)
}

// networkError wraps a net.Error into a descriptive error.
func networkError(provider string, err error) error {
	var netErr net.Error
	if errors.As(err, &netErr) {
		return fmt.Errorf("provider %s: network error: %w", provider, err)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("provider %s: DeadlineExceeded: %w", provider, err)
	}
	return fmt.Errorf("provider %s: %w", provider, err)
}
