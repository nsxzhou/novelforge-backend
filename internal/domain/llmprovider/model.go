package llmprovider

import "time"

// LLMProvider represents a persisted LLM provider configuration.
type LLMProvider struct {
	ID         string
	Provider   string
	Model      string
	BaseURL    string
	APIKey     string
	TimeoutSec int
	Priority   int
	Enabled    bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
