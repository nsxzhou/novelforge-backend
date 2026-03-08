package app

import (
	"fmt"
	"os"

	httpinfra "novelforge/backend/internal/infra/http"
	"novelforge/backend/internal/infra/llm"
	"novelforge/backend/internal/infra/storage"
	assetservice "novelforge/backend/internal/service/asset"
	projectservice "novelforge/backend/internal/service/project"
	"novelforge/backend/pkg/config"

	"github.com/cloudwego/hertz/pkg/app/server"
)

// Bootstrap 为后端服务连接运行时依赖。
type Bootstrap struct {
	Config       *config.AppConfig
	HTTP         *server.Hertz
	LLMClient    llm.Client
	Repositories *storage.Repositories
}

// LoadBootstrap 初始化运行时配置和基础设施。
func LoadBootstrap(configPath string) (*Bootstrap, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	repositories, err := storage.NewRepositories(cfg.Storage)
	if err != nil {
		return nil, fmt.Errorf("init repositories: %w", err)
	}

	if _, exists := os.LookupEnv(cfg.LLM.APIKeyEnv); !exists {
		_ = repositories.Close()
		return nil, fmt.Errorf("required environment variable %q is not set", cfg.LLM.APIKeyEnv)
	}

	llmClient, err := llm.NewClient(cfg.LLM)
	if err != nil {
		_ = repositories.Close()
		return nil, fmt.Errorf("init llm client: %w", err)
	}

	projectUseCase := projectservice.NewUseCase(projectservice.Dependencies{Projects: repositories.Projects})
	assetUseCase := assetservice.NewUseCase(assetservice.Dependencies{
		Assets:   repositories.Assets,
		Projects: repositories.Projects,
	})

	httpServer := httpinfra.NewServer(cfg.Server, httpinfra.Dependencies{
		Projects:  projectUseCase,
		Assets:    assetUseCase,
		Readiness: repositories,
	})

	return &Bootstrap{
		Config:       cfg,
		HTTP:         httpServer,
		LLMClient:    llmClient,
		Repositories: repositories,
	}, nil
}

// Close 释放引导(bootstrap)资源。
func (b *Bootstrap) Close() error {
	if b == nil || b.Repositories == nil {
		return nil
	}
	return b.Repositories.Close()
}
