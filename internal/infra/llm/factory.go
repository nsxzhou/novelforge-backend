package llm

import (
	"fmt"

	"novelforge/backend/pkg/config"
)

// NewClient creates an LLM client placeholder from runtime config.
func NewClient(cfg config.LLMConfig) (Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate llm config: %w", err)
	}

	return &placeholderClient{
		provider: cfg.Provider,
		model:    cfg.Model,
		chat:     nil,
	}, nil
}
