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
	APIKey      string `json:"-"`            // provider-specific key — never serialized
	TokenBudget int    `json:"token_budget"` // CLARION_LLM_TOKEN_BUDGET; default 100000
}

// apiKeyEnvVar returns the conventional environment variable name for the
// API key of the given provider (e.g. "OPENAI_API_KEY" for "openai").
func apiKeyEnvVar(provider string) string {
	switch provider {
	case "openai":
		return "OPENAI_API_KEY"
	case "anthropic":
		return "ANTHROPIC_API_KEY"
	case "gemini":
		return "GEMINI_API_KEY"
	default:
		return ""
	}
}

// LoadConfig reads Config from environment variables.
// All validation errors are accumulated into a single multi-line error.
func LoadConfig() (Config, error) {
	cfg := Config{
		Provider:    os.Getenv("CLARION_LLM_PROVIDER"),
		Model:       os.Getenv("CLARION_LLM_MODEL"),
		TokenBudget: 100000,
	}

	var errs []string

	switch cfg.Provider {
	case "":
		errs = append(errs, "CLARION_LLM_PROVIDER is required (openai, anthropic, or gemini)")
	case "openai", "anthropic", "gemini":
		keyVar := apiKeyEnvVar(cfg.Provider)
		cfg.APIKey = os.Getenv(keyVar)
		if cfg.APIKey == "" {
			errs = append(errs, keyVar+" is required")
		}
	default:
		errs = append(errs, "CLARION_LLM_PROVIDER must be one of: openai, anthropic, gemini")
	}

	if cfg.Model == "" {
		errs = append(errs, "CLARION_LLM_MODEL is required")
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
