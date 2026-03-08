package llm

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	openaimodel "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"novelforge/backend/internal/domain/generation"
	"novelforge/backend/pkg/config"
)

type stubToolCallingChatModel struct{}

func (s *stubToolCallingChatModel) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	return nil, nil
}

func (s *stubToolCallingChatModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, nil
}

func (s *stubToolCallingChatModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return s, nil
}

func validLLMConfig(apiKeyEnv string) config.LLMConfig {
	return config.LLMConfig{
		Provider:       config.LLMProviderOpenAICompatible,
		Model:          "gpt-4o-mini",
		BaseURL:        "https://api.openai.com/v1",
		APIKeyEnv:      apiKeyEnv,
		TimeoutSeconds: 60,
		Prompts: config.PromptConfig{
			generation.KindAssetGeneration:   "asset_generation.yaml",
			generation.KindChapterGeneration: "chapter_generation.yaml",
		},
	}
}

func TestNewClientOpenAICompatibleReturnsChatModel(t *testing.T) {
	const apiKeyEnv = "NOVELFORGE_LLM_API_KEY_FACTORY_SUCCESS_TEST"

	t.Setenv(apiKeyEnv, "test-key")

	var gotCfg *openaimodel.ChatModelConfig
	previousFactory := newOpenAIChatModel
	newOpenAIChatModel = func(_ context.Context, cfg *openaimodel.ChatModelConfig) (model.ToolCallingChatModel, error) {
		gotCfg = cfg
		return &stubToolCallingChatModel{}, nil
	}
	defer func() { newOpenAIChatModel = previousFactory }()

	client, err := NewClient(validLLMConfig(apiKeyEnv))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("NewClient() returned nil client")
	}
	if client.Provider() != config.LLMProviderOpenAICompatible {
		t.Fatalf("Provider() = %q, want %q", client.Provider(), config.LLMProviderOpenAICompatible)
	}
	if client.Model() != "gpt-4o-mini" {
		t.Fatalf("Model() = %q, want %q", client.Model(), "gpt-4o-mini")
	}
	if client.ChatModel() == nil {
		t.Fatal("ChatModel() = nil, want non-nil")
	}
	if gotCfg == nil {
		t.Fatal("newOpenAIChatModel() was not called")
	}
	if gotCfg.APIKey != "test-key" {
		t.Fatalf("ChatModelConfig.APIKey = %q, want %q", gotCfg.APIKey, "test-key")
	}
	if gotCfg.Model != "gpt-4o-mini" {
		t.Fatalf("ChatModelConfig.Model = %q, want %q", gotCfg.Model, "gpt-4o-mini")
	}
	if gotCfg.BaseURL != "https://api.openai.com/v1" {
		t.Fatalf("ChatModelConfig.BaseURL = %q, want %q", gotCfg.BaseURL, "https://api.openai.com/v1")
	}
	if gotCfg.Timeout != time.Minute {
		t.Fatalf("ChatModelConfig.Timeout = %v, want %v", gotCfg.Timeout, time.Minute)
	}
}

func TestNewClientOpenAICompatibleFactoryError(t *testing.T) {
	const apiKeyEnv = "NOVELFORGE_LLM_API_KEY_FACTORY_ERROR_TEST"

	t.Setenv(apiKeyEnv, "test-key")

	previousFactory := newOpenAIChatModel
	newOpenAIChatModel = func(_ context.Context, _ *openaimodel.ChatModelConfig) (model.ToolCallingChatModel, error) {
		return nil, errors.New("boom")
	}
	defer func() { newOpenAIChatModel = previousFactory }()

	_, err := NewClient(validLLMConfig(apiKeyEnv))
	if err == nil {
		t.Fatal("NewClient() error = nil, want constructor error")
	}
}

func TestNewClientMissingAPIKeyEnv(t *testing.T) {
	const apiKeyEnv = "NOVELFORGE_LLM_API_KEY_FACTORY_MISSING_TEST"

	_, err := NewClient(validLLMConfig(apiKeyEnv))
	if err == nil {
		t.Fatal("NewClient() error = nil, want missing env error")
	}
	if !strings.Contains(err.Error(), "required environment variable \""+apiKeyEnv+"\" is not set or empty") {
		t.Fatalf("NewClient() error = %v, want missing env error", err)
	}
}

func TestNewClientEmptyAPIKeyEnv(t *testing.T) {
	const apiKeyEnv = "NOVELFORGE_LLM_API_KEY_FACTORY_EMPTY_TEST"

	t.Setenv(apiKeyEnv, "   ")

	_, err := NewClient(validLLMConfig(apiKeyEnv))
	if err == nil {
		t.Fatal("NewClient() error = nil, want empty env error")
	}
	if !strings.Contains(err.Error(), "required environment variable \""+apiKeyEnv+"\" is not set or empty") {
		t.Fatalf("NewClient() error = %v, want empty env error", err)
	}
}
