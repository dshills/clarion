package cli

import (
	"encoding/json"
	"fmt"
	"os"
)

// Metrics holds per-command observability data.
type Metrics struct {
	TokensUsed           int     `json:"tokens_used"`
	EstimatedCost        float64 `json:"estimated_cost"`
	DurationMS           int64   `json:"duration_ms"`
	VerificationFailures int     `json:"verification_failures"`
}

// Print writes the metrics as a JSON object to stdout if --emit-metrics is set.
// EstimatedCost is computed from TokensUsed at print time (conservative $0.01/1K tokens).
func (m Metrics) Print() {
	if !flagEmitMetrics {
		return
	}
	m.EstimatedCost = float64(m.TokensUsed) / 1000 * 0.01
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		fmt.Fprintf(os.Stderr, "WARN: failed to write metrics: %v\n", err)
	}
}
