package storage

import (
	"fmt"

	"novelforge/backend/internal/infra/storage/memory"
	"novelforge/backend/pkg/config"
)

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
		}, nil
	default:
		return nil, fmt.Errorf("unsupported storage provider %q", cfg.Provider)
	}
}
