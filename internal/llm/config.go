package llm

import (
	"errors"
	"os"
	"strconv"
	"strings"
)

// Config holds LLM provider configuration loaded from environment variables.
type Config struct {
	Provider    string `json:"provider"`     // CLARION_LLM_PROVIDER
	Model       string `json:"model"`        // CLARION_LLM_MODEL
	APIKey      string `json:"-"`            // CLARION_LLM_API_KEY — never serialized
	TokenBudget int    `json:"token_budget"` // CLARION_LLM_TOKEN_BUDGET; default 100000
}

// LoadConfig reads Config from environment variables.
// All validation errors are accumulated into a single multi-line error.
func LoadConfig() (Config, error) {
	cfg := Config{
		Provider:    os.Getenv("CLARION_LLM_PROVIDER"),
		Model:       os.Getenv("CLARION_LLM_MODEL"),
		APIKey:      os.Getenv("CLARION_LLM_API_KEY"),
		TokenBudget: 100000,
	}

	var errs []string

	if cfg.Provider == "" {
		errs = append(errs, "CLARION_LLM_PROVIDER is required (openai or anthropic)")
	} else if cfg.Provider != "openai" && cfg.Provider != "anthropic" {
		errs = append(errs, "CLARION_LLM_PROVIDER must be one of: openai, anthropic")
	}

	if cfg.Model == "" {
		errs = append(errs, "CLARION_LLM_MODEL is required")
	}

	if cfg.APIKey == "" {
		errs = append(errs, "CLARION_LLM_API_KEY is required")
	}

	if raw := os.Getenv("CLARION_LLM_TOKEN_BUDGET"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			errs = append(errs, "CLARION_LLM_TOKEN_BUDGET must be an integer")
		} else if n <= 0 {
			errs = append(errs, "CLARION_LLM_TOKEN_BUDGET must be > 0")
		} else {
			cfg.TokenBudget = n
		}
	}

	if len(errs) > 0 {
		return Config{}, errors.New(strings.Join(errs, "\n"))
	}
	return cfg, nil
}
