package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"

	"novelforge/backend/internal/infra/llm"
	"novelforge/backend/internal/infra/llm/prompts"
	"novelforge/backend/internal/infra/storage"
	"novelforge/backend/pkg/config"
)

func writeTestConfig(t *testing.T, llmBlock string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "server:\n" +
		"  host: \"127.0.0.1\"\n" +
		"  port: 18080\n" +
		"  read_timeout_seconds: 1\n" +
		"  write_timeout_seconds: 1\n" +
		"storage:\n" +
		"  provider: \"memory\"\n" +
		llmBlock
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	return path
}

func validLLMConfigBlock(apiKeyEnv string) string {
	return "llm:\n" +
		"  provider: \"openai_compatible\"\n" +
		"  model: \"gpt-4o-mini\"\n" +
		"  base_url: \"https://api.openai.com/v1\"\n" +
		"  api_key_env: \"" + apiKeyEnv + "\"\n" +
		"  timeout_seconds: 60\n" +
		"  prompts:\n" +
		"    asset_generation: \"asset_generation.yaml\"\n" +
		"    chapter_generation: \"chapter_generation.yaml\"\n" +
		"    chapter_continuation: \"chapter_continuation.yaml\"\n" +
		"    chapter_rewrite: \"chapter_rewrite.yaml\"\n" +
		"    project_refinement: \"project_refinement.yaml\"\n" +
		"    asset_refinement: \"asset_refinement.yaml\"\n"
}

type stubLLMClient struct{}

func (s *stubLLMClient) Provider() string { return config.LLMProviderOpenAICompatible }

func (s *stubLLMClient) Model() string { return "gpt-4o-mini" }

func (s *stubLLMClient) ChatModel() model.ToolCallingChatModel { return nil }

func TestLoadBootstrapMissingAPIKeyEnv(t *testing.T) {
	const apiKeyEnv = "NOVELFORGE_LLM_API_KEY_BOOTSTRAP_MISSING_TEST"

	configPath := writeTestConfig(t, validLLMConfigBlock(apiKeyEnv))
	bootstrap, err := LoadBootstrap(configPath)
	if err == nil {
		if bootstrap != nil {
			_ = bootstrap.Close()
		}
		t.Fatal("LoadBootstrap() error = nil, want missing env error")
	}
	if !strings.Contains(err.Error(), "required environment variable \""+apiKeyEnv+"\" is not set or empty") {
		t.Fatalf("LoadBootstrap() error = %v, want missing env error", err)
	}
}

func TestLoadBootstrapEmptyAPIKeyEnv(t *testing.T) {
	const apiKeyEnv = "NOVELFORGE_LLM_API_KEY_BOOTSTRAP_EMPTY_TEST"

	t.Setenv(apiKeyEnv, "   ")

	configPath := writeTestConfig(t, validLLMConfigBlock(apiKeyEnv))
	bootstrap, err := LoadBootstrap(configPath)
	if err == nil {
		if bootstrap != nil {
			_ = bootstrap.Close()
		}
		t.Fatal("LoadBootstrap() error = nil, want empty env error")
	}
	if !strings.Contains(err.Error(), "required environment variable \""+apiKeyEnv+"\" is not set or empty") {
		t.Fatalf("LoadBootstrap() error = %v, want empty env error", err)
	}
}

func TestLoadBootstrapPromptStoreErrorClosesRepositories(t *testing.T) {
	const apiKeyEnv = "NOVELFORGE_LLM_API_KEY_BOOTSTRAP_PROMPT_TEST"

	t.Setenv(apiKeyEnv, "test-key")

	previousRunMigrations := runMigrations
	runMigrations = func(context.Context, config.StorageConfig) error { return nil }
	defer func() { runMigrations = previousRunMigrations }()

	repositories := &storage.Repositories{}
	previousNewRepositories := newRepositories
	newRepositories = func(_ config.StorageConfig) (*storage.Repositories, error) {
		return repositories, nil
	}
	defer func() { newRepositories = previousNewRepositories }()

	previousNewLLMClient := newLLMClient
	newLLMClient = func(_ config.LLMConfig) (llm.Client, error) {
		return &stubLLMClient{}, nil
	}
	defer func() { newLLMClient = previousNewLLMClient }()

	previousLoadPromptStore := loadPromptStore
	loadPromptStore = func(_ config.PromptConfig) (*prompts.Store, error) {
		return nil, errors.New("bad prompt config")
	}
	defer func() { loadPromptStore = previousLoadPromptStore }()

	closed := false
	previousCloseRepositories := closeRepositories
	closeRepositories = func(got *storage.Repositories) error {
		if got != repositories {
			t.Fatalf("closeRepositories() got %p, want %p", got, repositories)
		}
		closed = true
		return nil
	}
	defer func() { closeRepositories = previousCloseRepositories }()

	configPath := writeTestConfig(t, validLLMConfigBlock(apiKeyEnv))
	bootstrap, err := LoadBootstrap(configPath)
	if err == nil {
		if bootstrap != nil {
			_ = bootstrap.Close()
		}
		t.Fatal("LoadBootstrap() error = nil, want prompt store error")
	}
	if !strings.Contains(err.Error(), "load prompt store: bad prompt config") {
		t.Fatalf("LoadBootstrap() error = %v, want prompt store error", err)
	}
	if !closed {
		t.Fatal("closeRepositories() was not called")
	}
}

func TestLoadBootstrapSuccessWiresPromptStore(t *testing.T) {
	const apiKeyEnv = "NOVELFORGE_LLM_API_KEY_BOOTSTRAP_SUCCESS_TEST"

	t.Setenv(apiKeyEnv, "test-key")

	previousRunMigrations := runMigrations
	runMigrations = func(context.Context, config.StorageConfig) error { return nil }
	defer func() { runMigrations = previousRunMigrations }()

	configPath := writeTestConfig(t, validLLMConfigBlock(apiKeyEnv))
	bootstrap, err := LoadBootstrap(configPath)
	if err != nil {
		t.Fatalf("LoadBootstrap() error = %v", err)
	}
	defer bootstrap.Close()

	if bootstrap.LLMClient == nil {
		t.Fatal("LLMClient = nil, want non-nil")
	}
	if bootstrap.PromptStore == nil {
		t.Fatal("PromptStore = nil, want non-nil")
	}
	if _, ok := bootstrap.PromptStore.Get(config.PromptCapabilityAssetGeneration); !ok {
		t.Fatal("PromptStore.Get(asset_generation) = false, want true")
	}
	if _, ok := bootstrap.PromptStore.Get(config.PromptCapabilityProjectRefinement); !ok {
		t.Fatal("PromptStore.Get(project_refinement) = false, want true")
	}
}

func TestLoadBootstrapRunMigrationsError(t *testing.T) {
	const apiKeyEnv = "NOVELFORGE_LLM_API_KEY_BOOTSTRAP_MIGRATION_ERROR_TEST"

	t.Setenv(apiKeyEnv, "test-key")

	previousRunMigrations := runMigrations
	runMigrations = func(context.Context, config.StorageConfig) error {
		return errors.New("migration failed")
	}
	defer func() { runMigrations = previousRunMigrations }()

	previousNewRepositories := newRepositories
	newRepositories = func(config.StorageConfig) (*storage.Repositories, error) {
		t.Fatal("newRepositories() should not be called when migrations fail")
		return nil, nil
	}
	defer func() { newRepositories = previousNewRepositories }()

	configPath := writeTestConfig(t, validLLMConfigBlock(apiKeyEnv))
	bootstrap, err := LoadBootstrap(configPath)
	if err == nil {
		if bootstrap != nil {
			_ = bootstrap.Close()
		}
		t.Fatal("LoadBootstrap() error = nil, want migration error")
	}
	if !strings.Contains(err.Error(), "run migrations: migration failed") {
		t.Fatalf("LoadBootstrap() error = %v, want migration error", err)
	}
}

func TestLoadBootstrapRunsMigrationsBeforeRepositories(t *testing.T) {
	const apiKeyEnv = "NOVELFORGE_LLM_API_KEY_BOOTSTRAP_MIGRATION_ORDER_TEST"

	t.Setenv(apiKeyEnv, "test-key")

	callOrder := make([]string, 0, 2)

	previousRunMigrations := runMigrations
	runMigrations = func(context.Context, config.StorageConfig) error {
		callOrder = append(callOrder, "migrate")
		return nil
	}
	defer func() { runMigrations = previousRunMigrations }()

	previousNewRepositories := newRepositories
	newRepositories = func(config.StorageConfig) (*storage.Repositories, error) {
		callOrder = append(callOrder, "repositories")
		return &storage.Repositories{}, nil
	}
	defer func() { newRepositories = previousNewRepositories }()

	previousNewLLMClient := newLLMClient
	newLLMClient = func(config.LLMConfig) (llm.Client, error) {
		return &stubLLMClient{}, nil
	}
	defer func() { newLLMClient = previousNewLLMClient }()

	previousLoadPromptStore := loadPromptStore
	loadPromptStore = prompts.LoadStore
	defer func() { loadPromptStore = previousLoadPromptStore }()

	configPath := writeTestConfig(t, validLLMConfigBlock(apiKeyEnv))
	bootstrap, err := LoadBootstrap(configPath)
	if err != nil {
		t.Fatalf("LoadBootstrap() error = %v", err)
	}
	defer bootstrap.Close()

	if len(callOrder) < 2 {
		t.Fatalf("call order = %#v, want at least migrate then repositories", callOrder)
	}
	if callOrder[0] != "migrate" || callOrder[1] != "repositories" {
		t.Fatalf("call order = %#v, want [migrate repositories ...]", callOrder)
	}
}
