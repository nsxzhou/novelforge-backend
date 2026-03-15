package llm

// ProviderConfig holds configuration for a single LLM provider.
type ProviderConfig struct {
	ID         string `json:"id"`
	Provider   string `json:"provider"`         // "openai_compatible"
	Model      string `json:"model"`
	BaseURL    string `json:"base_url"`
	APIKey     string `json:"api_key"`
	TimeoutSec int    `json:"timeout_seconds"`
	Priority   int    `json:"priority"`          // lower = higher priority, 0 = primary
	Enabled    bool   `json:"enabled"`
}
