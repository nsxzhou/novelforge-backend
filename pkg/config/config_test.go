package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validPromptConfig() PromptConfig {
	return PromptConfig{
		AssetGeneration:     "asset_generation.yaml",
		ChapterGeneration:   "chapter_generation.yaml",
		ChapterContinuation: "chapter_continuation.yaml",
		ChapterRewrite:      "chapter_rewrite.yaml",
		ProjectRefinement:   "project_refinement.yaml",
		AssetRefinement:     "asset_refinement.yaml",
	}
}

func validAppConfig() AppConfig {
	return AppConfig{
		Server: ServerConfig{
			Host:                "127.0.0.1",
			Port:                8080,
			ReadTimeoutSeconds:  15,
			WriteTimeoutSeconds: 15,
			CORS: CORSConfig{
				AllowedOrigins: []string{"http://127.0.0.1:5173"},
			},
		},
		Storage: StorageConfig{
			Provider: StorageProviderMemory,
		},
		LLM: LLMConfig{
			ProviderEnv:    "INKMUSE_LLM_PROVIDER",
			ModelEnv:       "INKMUSE_LLM_MODEL",
			BaseURLEnv:     "INKMUSE_LLM_BASE_URL",
			APIKeyEnv:      "INKMUSE_LLM_API_KEY",
			TimeoutSeconds: 60,
			Prompts:        validPromptConfig(),
		},
	}
}

func TestCORSConfigAllowedOriginsOrDefault(t *testing.T) {
	t.Run("returns configured origins with normalization", func(t *testing.T) {
		cfg := CORSConfig{
			AllowedOrigins: []string{
				" http://localhost:5173 ",
				"http://localhost:5173",
				"http://127.0.0.1:5173",
			},
		}

		got := cfg.AllowedOriginsOrDefault()
		want := []string{"http://localhost:5173", "http://127.0.0.1:5173"}
		if len(got) != len(want) {
			t.Fatalf("len(AllowedOriginsOrDefault()) = %d, want %d", len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("AllowedOriginsOrDefault()[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})

	t.Run("falls back to default origins when unset", func(t *testing.T) {
		cfg := CORSConfig{}

		got := cfg.AllowedOriginsOrDefault()
		want := []string{"http://localhost:5173", "http://127.0.0.1:5173"}
		if len(got) != len(want) {
			t.Fatalf("len(AllowedOriginsOrDefault()) = %d, want %d", len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("AllowedOriginsOrDefault()[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})
}

func TestCORSConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     CORSConfig
		wantErr string
	}{
		{
			name: "valid empty config",
			cfg:  CORSConfig{},
		},
		{
			name: "valid origins",
			cfg: CORSConfig{
				AllowedOrigins: []string{"http://localhost:5173", "http://127.0.0.1:5173"},
			},
		},
		{
			name: "contains empty origin",
			cfg: CORSConfig{
				AllowedOrigins: []string{"http://localhost:5173", "   "},
			},
			wantErr: "allowed_origins must not contain empty values",
		},
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

func TestStorageConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     StorageConfig
		wantErr string
	}{
		{name: "valid memory", cfg: StorageConfig{Provider: StorageProviderMemory}},
		{name: "valid postgres", cfg: StorageConfig{Provider: StorageProviderPostgres, Postgres: PostgresConfig{URLEnv: "INKMUSE_DATABASE_URL", MaxOpenConns: 10, MaxIdleConns: 5, ConnMaxLifetimeSeconds: 300}}},
		{name: "empty provider", cfg: StorageConfig{}, wantErr: "provider must not be empty"},
		{name: "unsupported provider", cfg: StorageConfig{Provider: "sqlite"}, wantErr: "provider must be \"memory\" or \"postgres\""},
		{name: "postgres missing url env", cfg: StorageConfig{Provider: StorageProviderPostgres}, wantErr: "invalid postgres config: url_env must not be empty"},
		{name: "postgres negative max open conns", cfg: StorageConfig{Provider: StorageProviderPostgres, Postgres: PostgresConfig{URLEnv: "INKMUSE_DATABASE_URL", MaxOpenConns: -1}}, wantErr: "max_open_conns must be greater than or equal to 0"},
		{name: "postgres idle larger than open", cfg: StorageConfig{Provider: StorageProviderPostgres, Postgres: PostgresConfig{URLEnv: "INKMUSE_DATABASE_URL", MaxOpenConns: 5, MaxIdleConns: 6}}, wantErr: "max_idle_conns must be less than or equal to max_open_conns"},
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
		{name: "missing asset generation prompt", cfg: PromptConfig{
			ChapterGeneration:   "chapter_generation.yaml",
			ChapterContinuation: "chapter_continuation.yaml",
			ChapterRewrite:      "chapter_rewrite.yaml",
			ProjectRefinement:   "project_refinement.yaml",
			AssetRefinement:     "asset_refinement.yaml",
		}, wantErr: "asset_generation must not be empty"},
		{name: "missing chapter generation prompt", cfg: PromptConfig{
			AssetGeneration:     "asset_generation.yaml",
			ChapterContinuation: "chapter_continuation.yaml",
			ChapterRewrite:      "chapter_rewrite.yaml",
			ProjectRefinement:   "project_refinement.yaml",
			AssetRefinement:     "asset_refinement.yaml",
		}, wantErr: "chapter_generation must not be empty"},
		{name: "missing chapter continuation prompt", cfg: PromptConfig{
			AssetGeneration:   "asset_generation.yaml",
			ChapterGeneration: "chapter_generation.yaml",
			ChapterRewrite:    "chapter_rewrite.yaml",
			ProjectRefinement: "project_refinement.yaml",
			AssetRefinement:   "asset_refinement.yaml",
		}, wantErr: "chapter_continuation must not be empty"},
		{name: "missing chapter rewrite prompt", cfg: PromptConfig{
			AssetGeneration:     "asset_generation.yaml",
			ChapterGeneration:   "chapter_generation.yaml",
			ChapterContinuation: "chapter_continuation.yaml",
			ProjectRefinement:   "project_refinement.yaml",
			AssetRefinement:     "asset_refinement.yaml",
		}, wantErr: "chapter_rewrite must not be empty"},
		{name: "missing project refinement prompt", cfg: PromptConfig{
			AssetGeneration:     "asset_generation.yaml",
			ChapterGeneration:   "chapter_generation.yaml",
			ChapterContinuation: "chapter_continuation.yaml",
			ChapterRewrite:      "chapter_rewrite.yaml",
			AssetRefinement:     "asset_refinement.yaml",
		}, wantErr: "project_refinement must not be empty"},
		{name: "missing asset refinement prompt", cfg: PromptConfig{
			AssetGeneration:     "asset_generation.yaml",
			ChapterGeneration:   "chapter_generation.yaml",
			ChapterContinuation: "chapter_continuation.yaml",
			ChapterRewrite:      "chapter_rewrite.yaml",
			ProjectRefinement:   "project_refinement.yaml",
		}, wantErr: "asset_refinement must not be empty"},
		{name: "empty filename", cfg: PromptConfig{
			AssetGeneration:     "asset_generation.yaml",
			ChapterGeneration:   "chapter_generation.yaml",
			ChapterContinuation: "chapter_continuation.yaml",
			ChapterRewrite:      "chapter_rewrite.yaml",
			ProjectRefinement:   "project_refinement.yaml",
			AssetRefinement:     "",
		}, wantErr: "asset_refinement must not be empty"},
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
			name: "valid with all env vars",
			cfg: LLMConfig{
				ProviderEnv:    "INKMUSE_LLM_PROVIDER",
				ModelEnv:       "INKMUSE_LLM_MODEL",
				BaseURLEnv:     "INKMUSE_LLM_BASE_URL",
				APIKeyEnv:      "INKMUSE_LLM_API_KEY",
				TimeoutSeconds: 60,
				Prompts:        validPromptConfig(),
			},
		},
		{name: "valid without env vars (optional)", cfg: LLMConfig{TimeoutSeconds: 60, Prompts: validPromptConfig()}},
		{name: "invalid timeout", cfg: LLMConfig{TimeoutSeconds: 0, Prompts: validPromptConfig()}, wantErr: "timeout_seconds must be greater than 0"},
		{name: "missing prompts", cfg: LLMConfig{TimeoutSeconds: 60}, wantErr: "invalid prompts config: asset_generation must not be empty"},
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

func TestServerConfigAddress(t *testing.T) {
	cfg := ServerConfig{Host: "127.0.0.1", Port: 18080}
	if got, want := cfg.Address(), "127.0.0.1:18080"; got != want {
		t.Fatalf("Address() = %q, want %q", got, want)
	}
}

func TestLoad(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		content := `
server:
  host: "127.0.0.1"
  port: 8080
  read_timeout_seconds: 10
  write_timeout_seconds: 10
storage:
  provider: "memory"
llm:
  provider_env: "INKMUSE_LLM_PROVIDER"
  model_env: "INKMUSE_LLM_MODEL"
  base_url_env: "INKMUSE_LLM_BASE_URL"
  api_key_env: "INKMUSE_LLM_API_KEY"
  timeout_seconds: 60
  prompts:
    asset_generation: "asset_generation.yaml"
    chapter_generation: "chapter_generation.yaml"
    chapter_continuation: "chapter_continuation.yaml"
    chapter_rewrite: "chapter_rewrite.yaml"
    project_refinement: "project_refinement.yaml"
    asset_refinement: "asset_refinement.yaml"
`
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		cfg, err := Load(path)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if cfg.Server.Host != "127.0.0.1" || cfg.Storage.Provider != StorageProviderMemory || cfg.LLM.ProviderEnv != "INKMUSE_LLM_PROVIDER" {
			t.Fatalf("Load() cfg = %#v, want parsed app config", cfg)
		}
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
		if err == nil {
			t.Fatal("Load() error = nil, want read config error")
		}
		if !strings.Contains(err.Error(), "read config file") {
			t.Fatalf("Load() error = %v, want read config wrapper", err)
		}
	})

	t.Run("invalid yaml", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "broken.yaml")
		if err := os.WriteFile(path, []byte("server:\n  host: [\n"), 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		_, err := Load(path)
		if err == nil {
			t.Fatal("Load() error = nil, want yaml unmarshal error")
		}
		if !strings.Contains(err.Error(), "unmarshal config yaml") {
			t.Fatalf("Load() error = %v, want yaml wrapper", err)
		}
	})

	t.Run("unknown prompt key", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "unknown-prompt.yaml")
		content := `
server:
  host: "127.0.0.1"
  port: 8080
  read_timeout_seconds: 10
  write_timeout_seconds: 10
storage:
  provider: "memory"
llm:
  provider_env: "INKMUSE_LLM_PROVIDER"
  model_env: "INKMUSE_LLM_MODEL"
  base_url_env: "INKMUSE_LLM_BASE_URL"
  api_key_env: "INKMUSE_LLM_API_KEY"
  timeout_seconds: 60
  prompts:
    asset_generation: "asset_generation.yaml"
    chapter_generation: "chapter_generation.yaml"
    chapter_continuation: "chapter_continuation.yaml"
    chapter_rewrite: "chapter_rewrite.yaml"
    project_refinement: "project_refinement.yaml"
    asset_refinement: "asset_refinement.yaml"
    unsupported_kind: "unsupported.yaml"
`
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		_, err := Load(path)
		if err == nil {
			t.Fatal("Load() error = nil, want unknown field error")
		}
		if !strings.Contains(err.Error(), "field unsupported_kind not found") {
			t.Fatalf("Load() error = %v, want unknown prompt field error", err)
		}
	})
}
