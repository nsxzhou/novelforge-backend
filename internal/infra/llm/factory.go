package llm

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	openaimodel "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"

	"novelforge/backend/pkg/config"
)

var newOpenAIChatModel = func(ctx context.Context, cfg *openaimodel.ChatModelConfig) (model.ToolCallingChatModel, error) {
	return openaimodel.NewChatModel(ctx, cfg)
}

// NewClient creates an LLM client from runtime config.
func NewClient(cfg config.LLMConfig) (Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate llm config: %w", err)
	}

	switch cfg.Provider {
	case config.LLMProviderOpenAICompatible:
		apiKey, exists := os.LookupEnv(cfg.APIKeyEnv)
		if !exists || strings.TrimSpace(apiKey) == "" {
			return nil, fmt.Errorf("required environment variable %q is not set or empty", cfg.APIKeyEnv)
		}

		chatModel, err := newOpenAIChatModel(context.Background(), &openaimodel.ChatModelConfig{
			APIKey:  apiKey,
			Model:   cfg.Model,
			BaseURL: cfg.BaseURL,
			Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second,
		})
		if err != nil {
			return nil, fmt.Errorf("init openai compatible chat model: %w", err)
		}

		return &openAICompatibleClient{
			provider: cfg.Provider,
			model:    cfg.Model,
			chat:     chatModel,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported llm provider %q", cfg.Provider)
	}
}
