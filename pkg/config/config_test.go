package config

import (
	"strings"
	"testing"
)

func validPromptConfig() PromptConfig {
	return PromptConfig{
		"asset_generation":   "asset_generation.yaml",
		"chapter_generation": "chapter_generation.yaml",
	}
}

func validAppConfig() AppConfig {
	return AppConfig{
		Server: ServerConfig{
			Host:                "127.0.0.1",
			Port:                8080,
			ReadTimeoutSeconds:  15,
			WriteTimeoutSeconds: 15,
		},
		Storage: StorageConfig{
			Provider: StorageProviderMemory,
		},
		LLM: LLMConfig{
			Provider:       LLMProviderOpenAICompatible,
			Model:          "gpt-4o-mini",
			BaseURL:        "https://api.openai.com/v1",
			APIKeyEnv:      "NOVELFORGE_LLM_API_KEY",
			TimeoutSeconds: 60,
			Prompts:        validPromptConfig(),
		},
	}
}

func TestStorageConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     StorageConfig
		wantErr string
	}{
		{name: "valid memory", cfg: StorageConfig{Provider: StorageProviderMemory}},
		{name: "valid postgres", cfg: StorageConfig{Provider: StorageProviderPostgres, Postgres: PostgresConfig{URLEnv: "NOVELFORGE_DATABASE_URL", MaxOpenConns: 10, MaxIdleConns: 5, ConnMaxLifetimeSeconds: 300}}},
		{name: "empty provider", cfg: StorageConfig{}, wantErr: "provider must not be empty"},
		{name: "unsupported provider", cfg: StorageConfig{Provider: "sqlite"}, wantErr: "provider must be \"memory\" or \"postgres\""},
		{name: "postgres missing url env", cfg: StorageConfig{Provider: StorageProviderPostgres}, wantErr: "invalid postgres config: url_env must not be empty"},
		{name: "postgres negative max open conns", cfg: StorageConfig{Provider: StorageProviderPostgres, Postgres: PostgresConfig{URLEnv: "NOVELFORGE_DATABASE_URL", MaxOpenConns: -1}}, wantErr: "max_open_conns must be greater than or equal to 0"},
		{name: "postgres idle larger than open", cfg: StorageConfig{Provider: StorageProviderPostgres, Postgres: PostgresConfig{URLEnv: "NOVELFORGE_DATABASE_URL", MaxOpenConns: 5, MaxIdleConns: 6}}, wantErr: "max_idle_conns must be less than or equal to max_open_conns"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() error = nil, want %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestPromptConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     PromptConfig
		wantErr string
	}{
		{name: "valid", cfg: validPromptConfig()},
		{name: "missing asset prompt", cfg: PromptConfig{"chapter_generation": "chapter_generation.yaml"}, wantErr: "asset_generation must not be empty"},
		{name: "missing chapter prompt", cfg: PromptConfig{"asset_generation": "asset_generation.yaml"}, wantErr: "chapter_generation must not be empty"},
		{name: "unsupported kind", cfg: PromptConfig{"asset_generation": "asset_generation.yaml", "chapter_generation": "chapter_generation.yaml", "unsupported": "unsupported.yaml"}, wantErr: "\"unsupported\" is not a supported prompt kind"},
		{name: "empty filename", cfg: PromptConfig{"asset_generation": "asset_generation.yaml", "chapter_generation": ""}, wantErr: "chapter_generation must not be empty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() error = nil, want %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestLLMConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     LLMConfig
		wantErr string
	}{
		{
			name: "valid openai compatible",
			cfg: LLMConfig{
				Provider:       LLMProviderOpenAICompatible,
				Model:          "gpt-4o-mini",
				BaseURL:        "https://api.openai.com/v1",
				APIKeyEnv:      "NOVELFORGE_LLM_API_KEY",
				TimeoutSeconds: 60,
				Prompts:        validPromptConfig(),
			},
		},
		{name: "empty provider", cfg: LLMConfig{}, wantErr: "provider must not be empty"},
		{name: "unsupported provider", cfg: LLMConfig{Provider: "placeholder", Model: "gpt-4o-mini", BaseURL: "https://api.openai.com/v1", APIKeyEnv: "NOVELFORGE_LLM_API_KEY", TimeoutSeconds: 60, Prompts: validPromptConfig()}, wantErr: "provider must be \"openai_compatible\""},
		{name: "missing model", cfg: LLMConfig{Provider: LLMProviderOpenAICompatible, BaseURL: "https://api.openai.com/v1", APIKeyEnv: "NOVELFORGE_LLM_API_KEY", TimeoutSeconds: 60, Prompts: validPromptConfig()}, wantErr: "model must not be empty"},
		{name: "missing base url", cfg: LLMConfig{Provider: LLMProviderOpenAICompatible, Model: "gpt-4o-mini", APIKeyEnv: "NOVELFORGE_LLM_API_KEY", TimeoutSeconds: 60, Prompts: validPromptConfig()}, wantErr: "base_url must not be empty"},
		{name: "missing api key env", cfg: LLMConfig{Provider: LLMProviderOpenAICompatible, Model: "gpt-4o-mini", BaseURL: "https://api.openai.com/v1", TimeoutSeconds: 60, Prompts: validPromptConfig()}, wantErr: "api_key_env must not be empty"},
		{name: "invalid timeout", cfg: LLMConfig{Provider: LLMProviderOpenAICompatible, Model: "gpt-4o-mini", BaseURL: "https://api.openai.com/v1", APIKeyEnv: "NOVELFORGE_LLM_API_KEY", TimeoutSeconds: 0, Prompts: validPromptConfig()}, wantErr: "timeout_seconds must be greater than 0"},
		{name: "missing prompts", cfg: LLMConfig{Provider: LLMProviderOpenAICompatible, Model: "gpt-4o-mini", BaseURL: "https://api.openai.com/v1", APIKeyEnv: "NOVELFORGE_LLM_API_KEY", TimeoutSeconds: 60}, wantErr: "invalid prompts config: asset_generation must not be empty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() error = nil, want %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestAppConfigValidateIncludesStorage(t *testing.T) {
	cfg := validAppConfig()
	cfg.Storage = StorageConfig{Provider: StorageProviderPostgres}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want storage validation error")
	}
	if !strings.Contains(err.Error(), "invalid storage config") {
		t.Fatalf("Validate() error = %v, want storage wrapper", err)
	}
}

func TestAppConfigValidateIncludesLLM(t *testing.T) {
	cfg := validAppConfig()
	cfg.LLM = LLMConfig{}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want llm validation error")
	}
	if !strings.Contains(err.Error(), "invalid llm config") {
		t.Fatalf("Validate() error = %v, want llm wrapper", err)
	}
}
