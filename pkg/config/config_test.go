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
		{name: "valid", cfg: StorageConfig{Provider: StorageProviderMemory}},
		{name: "empty provider", cfg: StorageConfig{}, wantErr: "provider must not be empty"},
		{name: "unsupported provider", cfg: StorageConfig{Provider: "sqlite"}, wantErr: "provider must be \"memory\""},
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
	cfg.Storage.Provider = "sqlite"

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want storage validation error")
	}
	if !strings.Contains(err.Error(), "invalid storage config") {
		t.Fatalf("Validate() error = %v, want storage wrapper", err)
	}
}
