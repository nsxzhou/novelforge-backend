package storage

import (
	"novelforge/backend/internal/domain/asset"
	"novelforge/backend/internal/domain/chapter"
	"novelforge/backend/internal/domain/conversation"
	"novelforge/backend/internal/domain/generation"
	"novelforge/backend/internal/domain/metric"
	"novelforge/backend/internal/domain/project"
)

// Repositories 将所有领域存储库(repository)契约组合在一起，用于运行时装配(runtime wiring)。
type Repositories struct {
	Projects          project.ProjectRepository
	Assets            asset.AssetRepository
	Chapters          chapter.ChapterRepository
	Conversations     conversation.ConversationRepository
	GenerationRecords generation.GenerationRecordRepository
	MetricEvents      metric.MetricEventRepository
}
