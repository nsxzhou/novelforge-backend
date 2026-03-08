package config

import (
	"strings"
	"testing"
)

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
			Provider:  "placeholder",
			Model:     "placeholder-model",
			APIKeyEnv: "NOVELFORGE_LLM_API_KEY",
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
