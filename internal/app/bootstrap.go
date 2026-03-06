package app

import (
	"fmt"
	"os"

	httpinfra "novelforge/backend/internal/infra/http"
	"novelforge/backend/internal/infra/llm"
	"novelforge/backend/pkg/config"

	"github.com/cloudwego/hertz/pkg/app/server"
)

// Bootstrap wires runtime dependencies for the backend service.
type Bootstrap struct {
	Config    *config.AppConfig
	HTTP      *server.Hertz
	LLMClient llm.Client
}

// LoadBootstrap initializes runtime config and infrastructure.
func LoadBootstrap(configPath string) (*Bootstrap, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	if _, exists := os.LookupEnv(cfg.LLM.APIKeyEnv); !exists {
		return nil, fmt.Errorf("required environment variable %q is not set", cfg.LLM.APIKeyEnv)
	}

	llmClient, err := llm.NewClient(cfg.LLM)
	if err != nil {
		return nil, fmt.Errorf("init llm client: %w", err)
	}

	httpServer := httpinfra.NewServer(cfg.Server)

	return &Bootstrap{
		Config:    cfg,
		HTTP:      httpServer,
		LLMClient: llmClient,
	}, nil
}
