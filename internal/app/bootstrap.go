package app

import (
	"context"
	"fmt"
	"log"

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
	seedFromEnv       = llm.SeedRegistryFromEnv
	newRegistryFromDB = llm.NewRegistryFromDB
	newRegistry       = llm.NewRegistry
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
	LLMRegistry  *llm.Registry
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

	llmRegistry, err := initLLMRegistry(cfg.LLM, repositories)
	if err != nil {
		_ = closeRepositories(repositories)
		return nil, fmt.Errorf("init llm registry: %w", err)
	}
	// Registry implements llm.Client, so it serves as both the Client and the Registry.
	var llmClient llm.Client = llmRegistry

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
		PromptOverrides:   repositories.PromptOverrides,
		Metrics:           metricUseCase,
	})
	chapterUseCase := chapterservice.NewUseCase(chapterservice.Dependencies{
		Chapters:          repositories.Chapters,
		Projects:          repositories.Projects,
		Assets:            repositories.Assets,
		GenerationRecords: repositories.GenerationRecords,
		LLMClient:         llmClient,
		PromptStore:       promptStore,
		PromptOverrides:   repositories.PromptOverrides,
		Metrics:           metricUseCase,
	})
	conversationUseCase := conversationservice.NewUseCase(conversationservice.Dependencies{
		Conversations:   repositories.Conversations,
		Projects:        repositories.Projects,
		Assets:          repositories.Assets,
		LLMClient:       llmClient,
		PromptStore:     promptStore,
		PromptOverrides: repositories.PromptOverrides,
		Metrics:         metricUseCase,
		TxRunner:        repositories.TxRunner,
	})

	httpServer := httpinfra.NewServer(cfg.Server, httpinfra.Dependencies{
		Projects:        projectUseCase,
		Assets:          assetUseCase,
		Chapters:        chapterUseCase,
		Conversations:   conversationUseCase,
		Readiness:       repositories,
		LLMRegistry:     llmRegistry,
		LLMProviders:    repositories.LLMProviders,
		PromptOverrides: repositories.PromptOverrides,
		PromptStore:     promptStore,
	})

	return &Bootstrap{
		Config:       cfg,
		HTTP:         httpServer,
		LLMClient:    llmClient,
		LLMRegistry:  llmRegistry,
		PromptStore:  promptStore,
		Repositories: repositories,
	}, nil
}

// initLLMRegistry creates the LLM Registry using a DB-first strategy:
// 1. Load providers from DB.
// 2. If DB has records, create Registry from DB (ignore env vars).
// 3. If DB is empty, try seeding from env vars and persist to DB.
// 4. If env vars are also empty, create an empty Registry.
func initLLMRegistry(cfg config.LLMConfig, repos *storage.Repositories) (*llm.Registry, error) {
	ctx := context.Background()

	// Step 1: Try loading from DB.
	if repos.LLMProviders != nil {
		dbProviders, err := repos.LLMProviders.List(ctx)
		if err != nil {
			return nil, fmt.Errorf("load providers from db: %w", err)
		}

		if len(dbProviders) > 0 {
			// Step 2: DB has records — use them.
			log.Printf("[bootstrap] loaded %d LLM provider(s) from database", len(dbProviders))
			return newRegistryFromDB(dbProviders)
		}
	}

	// Step 3: DB is empty — try seeding from env vars.
	seedConfigs, err := seedFromEnv(cfg)
	if err != nil {
		return nil, fmt.Errorf("seed from env: %w", err)
	}

	if len(seedConfigs) > 0 {
		// Persist seed to DB for future restarts.
		if repos.LLMProviders != nil {
			for _, sc := range seedConfigs {
				if persistErr := repos.LLMProviders.Upsert(ctx, llm.ProviderConfigToDomain(sc)); persistErr != nil {
					log.Printf("[bootstrap] warning: failed to persist seed provider %q to db: %v", sc.ID, persistErr)
				}
			}
		}
		log.Printf("[bootstrap] seeded %d LLM provider(s) from environment variables", len(seedConfigs))
		return newRegistry(seedConfigs)
	}

	// Step 4: No providers at all — create empty registry.
	log.Printf("[bootstrap] no LLM providers configured; system starts without LLM capability")
	return newRegistry(nil)
}

// Close 释放引导(bootstrap)资源。
func (b *Bootstrap) Close() error {
	if b == nil {
		return nil
	}
	return closeRepositories(b.Repositories)
}
