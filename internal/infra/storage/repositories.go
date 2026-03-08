package storage

import (
	"context"

	"novelforge/backend/internal/domain/asset"
	"novelforge/backend/internal/domain/chapter"
	"novelforge/backend/internal/domain/conversation"
	"novelforge/backend/internal/domain/generation"
	"novelforge/backend/internal/domain/metric"
	"novelforge/backend/internal/domain/project"
)

// ReadinessChecker reports whether the storage backend is ready.
type ReadinessChecker interface {
	CheckReadiness(ctx context.Context) error
}

// Repositories 将所有领域存储库(repository)契约组合在一起，用于运行时装配(runtime wiring)。
type Repositories struct {
	Projects          project.ProjectRepository
	Assets            asset.AssetRepository
	Chapters          chapter.ChapterRepository
	Conversations     conversation.ConversationRepository
	GenerationRecords generation.GenerationRecordRepository
	MetricEvents      metric.MetricEventRepository

	readiness ReadinessChecker
	closeFunc func() error
}

// CheckReadiness reports whether the configured storage backend is ready.
func (r *Repositories) CheckReadiness(ctx context.Context) error {
	if r == nil || r.readiness == nil {
		return nil
	}
	return r.readiness.CheckReadiness(ctx)
}

// Close releases storage resources held by the repository bundle.
func (r *Repositories) Close() error {
	if r == nil || r.closeFunc == nil {
		return nil
	}
	return r.closeFunc()
}
