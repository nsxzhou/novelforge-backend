package storage

import (
	"context"
	"fmt"

	"novelforge/backend/internal/infra/storage/memory"
	"novelforge/backend/internal/infra/storage/postgres"
	"novelforge/backend/pkg/config"
)

var newPostgresProvider = postgres.NewProvider

// NewRepositories 为配置的存储提供程序创建存储库(repository)实现。
func NewRepositories(cfg config.StorageConfig) (*Repositories, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate storage config: %w", err)
	}

	switch cfg.Provider {
	case config.StorageProviderMemory:
		return &Repositories{
			Projects:          memory.NewProjectRepository(),
			Assets:            memory.NewAssetRepository(),
			Chapters:          memory.NewChapterRepository(),
			Conversations:     memory.NewConversationRepository(),
			GenerationRecords: memory.NewGenerationRecordRepository(),
			MetricEvents:      memory.NewMetricEventRepository(),
			PromptOverrides:   memory.NewPromptOverrideRepository(),
			LLMProviders:      memory.NewLLMProviderRepository(),
			TxRunner:          noopTxRunner{},
		}, nil
	case config.StorageProviderPostgres:
		provider, err := newPostgresProvider(context.Background(), cfg.Postgres)
		if err != nil {
			return nil, fmt.Errorf("init postgres provider: %w", err)
		}

		return &Repositories{
			Projects:          postgres.NewProjectRepository(provider.DB()),
			Assets:            postgres.NewAssetRepository(provider.DB()),
			Chapters:          postgres.NewChapterRepository(provider.DB()),
			Conversations:     postgres.NewConversationRepository(provider.DB()),
			GenerationRecords: postgres.NewGenerationRecordRepository(provider.DB()),
			MetricEvents:      postgres.NewMetricEventRepository(provider.DB()),
			PromptOverrides:   postgres.NewPromptOverrideRepository(provider.DB()),
			LLMProviders:      postgres.NewLLMProviderRepository(provider.DB()),
			TxRunner:          postgres.NewTxRunner(provider.DB()),
			readiness:         provider,
			closeFunc:         provider.Close,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported storage provider %q", cfg.Provider)
	}
}

type noopTxRunner struct{}

func (noopTxRunner) InTx(ctx context.Context, fn func(context.Context) error) error {
	if fn == nil {
		return nil
	}
	return fn(ctx)
}
