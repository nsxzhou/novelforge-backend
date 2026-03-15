package llm

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	openaimodel "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"

	"inkmuse/backend/internal/domain/llmprovider"
	"inkmuse/backend/pkg/config"
)

var newOpenAIChatModel = func(ctx context.Context, cfg *openaimodel.ChatModelConfig) (model.ToolCallingChatModel, error) {
	return openaimodel.NewChatModel(ctx, cfg)
}

// NewClient creates an LLM client from runtime config.
// The returned Client is backed by a single-provider Registry for backward compatibility.
func NewClient(cfg config.LLMConfig) (Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate llm config: %w", err)
	}

	// 运行时 provider/model/base_url 统一从环境变量读取，避免把具体值写入配置文件。
	provider, exists := os.LookupEnv(cfg.ProviderEnv)
	if !exists || strings.TrimSpace(provider) == "" {
		return nil, fmt.Errorf("required environment variable %q is not set or empty", cfg.ProviderEnv)
	}
	provider = strings.TrimSpace(provider)

	modelName, exists := os.LookupEnv(cfg.ModelEnv)
	if !exists || strings.TrimSpace(modelName) == "" {
		return nil, fmt.Errorf("required environment variable %q is not set or empty", cfg.ModelEnv)
	}
	modelName = strings.TrimSpace(modelName)

	baseURL, exists := os.LookupEnv(cfg.BaseURLEnv)
	if !exists || strings.TrimSpace(baseURL) == "" {
		return nil, fmt.Errorf("required environment variable %q is not set or empty", cfg.BaseURLEnv)
	}
	baseURL = strings.TrimSpace(baseURL)

	switch provider {
	case config.LLMProviderOpenAICompatible:
		apiKey, exists := os.LookupEnv(cfg.APIKeyEnv)
		if !exists || strings.TrimSpace(apiKey) == "" {
			return nil, fmt.Errorf("required environment variable %q is not set or empty", cfg.APIKeyEnv)
		}

		chatModel, err := newOpenAIChatModel(context.Background(), &openaimodel.ChatModelConfig{
			APIKey:  apiKey,
			Model:   modelName,
			BaseURL: baseURL,
			Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second,
		})
		if err != nil {
			return nil, fmt.Errorf("init openai compatible chat model: %w", err)
		}

		return &openAICompatibleClient{
			provider: provider,
			model:    modelName,
			chat:     chatModel,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported llm provider %q", provider)
	}
}

// NewClientAsRegistry creates an LLM Registry from runtime config.
// The existing env var config becomes the default primary provider wrapped in a Registry.
func NewClientAsRegistry(cfg config.LLMConfig) (*Registry, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate llm config: %w", err)
	}

	seed, err := SeedRegistryFromEnv(cfg)
	if err != nil {
		return nil, err
	}
	if len(seed) == 0 {
		return nil, fmt.Errorf("required environment variable %q is not set or empty", cfg.ProviderEnv)
	}
	return NewRegistry(seed)
}

// SeedRegistryFromEnv reads environment variables and returns a seed ProviderConfig list.
// Returns an empty list (not an error) when the environment variables are not set.
// Returns an error only if the configuration format is invalid (e.g. unsupported provider type).
func SeedRegistryFromEnv(cfg config.LLMConfig) ([]ProviderConfig, error) {
	provider := strings.TrimSpace(os.Getenv(cfg.ProviderEnv))
	if provider == "" {
		return nil, nil
	}

	modelName := strings.TrimSpace(os.Getenv(cfg.ModelEnv))
	if modelName == "" {
		return nil, nil
	}

	baseURL := strings.TrimSpace(os.Getenv(cfg.BaseURLEnv))
	if baseURL == "" {
		return nil, nil
	}

	apiKey := strings.TrimSpace(os.Getenv(cfg.APIKeyEnv))
	if apiKey == "" {
		return nil, nil
	}

	if provider != config.LLMProviderOpenAICompatible {
		return nil, fmt.Errorf("unsupported llm provider %q", provider)
	}

	return []ProviderConfig{{
		ID:         "default",
		Provider:   provider,
		Model:      modelName,
		BaseURL:    baseURL,
		APIKey:     apiKey,
		TimeoutSec: cfg.TimeoutSeconds,
		Priority:   0,
		Enabled:    true,
	}}, nil
}

// NewRegistryFromDB creates a Registry from persisted LLMProvider records.
func NewRegistryFromDB(providers []*llmprovider.LLMProvider) (*Registry, error) {
	configs := make([]ProviderConfig, len(providers))
	for i, p := range providers {
		configs[i] = ProviderConfig{
			ID:         p.ID,
			Provider:   p.Provider,
			Model:      p.Model,
			BaseURL:    p.BaseURL,
			APIKey:     p.APIKey,
			TimeoutSec: p.TimeoutSec,
			Priority:   p.Priority,
			Enabled:    p.Enabled,
		}
	}
	return NewRegistry(configs)
}

// ProviderConfigToDomain converts a ProviderConfig to a domain LLMProvider.
func ProviderConfigToDomain(cfg ProviderConfig) *llmprovider.LLMProvider {
	return &llmprovider.LLMProvider{
		ID:         cfg.ID,
		Provider:   cfg.Provider,
		Model:      cfg.Model,
		BaseURL:    cfg.BaseURL,
		APIKey:     cfg.APIKey,
		TimeoutSec: cfg.TimeoutSec,
		Priority:   cfg.Priority,
		Enabled:    cfg.Enabled,
	}
}
