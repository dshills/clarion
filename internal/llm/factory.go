package llm

import (
	"fmt"
	"net/http"
	"time"
)

// NewAdapter creates a ProviderAdapter from the given Config.
func NewAdapter(cfg Config) (ProviderAdapter, error) {
	client := &http.Client{Timeout: 120 * time.Second}

	var adapter ProviderAdapter
	switch cfg.Provider {
	case "openai":
		adapter = &openAIAdapter{
			model:      cfg.Model,
			apiKey:     cfg.APIKey,
			client:     client,
			retryDelay: 2 * time.Second,
		}
	case "anthropic":
		adapter = &anthropicAdapter{
			model:      cfg.Model,
			apiKey:     cfg.APIKey,
			client:     client,
			retryDelay: 2 * time.Second,
		}
	default:
		return nil, fmt.Errorf("unknown provider: %q (must be openai or anthropic)", cfg.Provider)
	}

	if err := adapter.Validate(); err != nil {
		return nil, err
	}
	return adapter, nil
}
