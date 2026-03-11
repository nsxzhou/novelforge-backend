package app

import (
	"context"
	"fmt"
	"os"
	"strings"

	httpinfra "novelforge/backend/internal/infra/http"
	"novelforge/backend/internal/infra/llm"
	"novelforge/backend/internal/infra/llm/prompts"
	"novelforge/backend/internal/infra/storage"
	assetservice "novelforge/backend/internal/service/asset"
	chapterservice "novelforge/backend/internal/service/chapter"
	conversationservice "novelforge/backend/internal/service/conversation"
	metricservice "novelforge/backend/internal/service/metric"
	projectservice "novelforge/backend/internal/service/project"
	"novelforge/backend/pkg/config"

	"github.com/cloudwego/hertz/pkg/app/server"
)

var (
	newRepositories   = storage.NewRepositories
	runMigrations     = storage.RunMigrations
	newLLMClient      = llm.NewClient
	loadPromptStore   = prompts.LoadStore
	closeRepositories = func(repositories *storage.Repositories) error {
		if repositories == nil {
			return nil
		}
		return repositories.Close()
	}
)

// Bootstrap 为后端服务连接运行时依赖。
type Bootstrap struct {
	Config       *config.AppConfig
	HTTP         *server.Hertz
	LLMClient    llm.Client
	PromptStore  *prompts.Store
	Repositories *storage.Repositories
}

// LoadBootstrap 初始化运行时配置和基础设施。
func LoadBootstrap(configPath string) (*Bootstrap, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	if err := runMigrations(context.Background(), cfg.Storage); err != nil {
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	repositories, err := newRepositories(cfg.Storage)
	if err != nil {
		return nil, fmt.Errorf("init repositories: %w", err)
	}

	apiKey, exists := os.LookupEnv(cfg.LLM.APIKeyEnv)
	if !exists || strings.TrimSpace(apiKey) == "" {
		_ = closeRepositories(repositories)
		return nil, fmt.Errorf("required environment variable %q is not set or empty", cfg.LLM.APIKeyEnv)
	}

	llmClient, err := newLLMClient(cfg.LLM)
	if err != nil {
		_ = closeRepositories(repositories)
		return nil, fmt.Errorf("init llm client: %w", err)
	}

	promptStore, err := loadPromptStore(cfg.LLM.Prompts)
	if err != nil {
		_ = closeRepositories(repositories)
		return nil, fmt.Errorf("load prompt store: %w", err)
	}

	metricUseCase := metricservice.NewUseCase(metricservice.Dependencies{
		MetricEvents: repositories.MetricEvents,
	})
	projectUseCase := projectservice.NewUseCase(projectservice.Dependencies{Projects: repositories.Projects})
	assetUseCase := assetservice.NewUseCase(assetservice.Dependencies{
		Assets:            repositories.Assets,
		Projects:          repositories.Projects,
		GenerationRecords: repositories.GenerationRecords,
		LLMClient:         llmClient,
		PromptStore:       promptStore,
		Metrics:           metricUseCase,
	})
	chapterUseCase := chapterservice.NewUseCase(chapterservice.Dependencies{
		Chapters:          repositories.Chapters,
		Projects:          repositories.Projects,
		Assets:            repositories.Assets,
		GenerationRecords: repositories.GenerationRecords,
		LLMClient:         llmClient,
		PromptStore:       promptStore,
		Metrics:           metricUseCase,
	})
	conversationUseCase := conversationservice.NewUseCase(conversationservice.Dependencies{
		Conversations: repositories.Conversations,
		Projects:      repositories.Projects,
		Assets:        repositories.Assets,
		LLMClient:     llmClient,
		PromptStore:   promptStore,
		Metrics:       metricUseCase,
		TxRunner:      repositories.TxRunner,
	})

	httpServer := httpinfra.NewServer(cfg.Server, httpinfra.Dependencies{
		Projects:      projectUseCase,
		Assets:        assetUseCase,
		Chapters:      chapterUseCase,
		Conversations: conversationUseCase,
		Readiness:     repositories,
	})

	return &Bootstrap{
		Config:       cfg,
		HTTP:         httpServer,
		LLMClient:    llmClient,
		PromptStore:  promptStore,
		Repositories: repositories,
	}, nil
}

// Close 释放引导(bootstrap)资源。
func (b *Bootstrap) Close() error {
	if b == nil {
		return nil
	}
	return closeRepositories(b.Repositories)
}
