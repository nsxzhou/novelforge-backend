package storage

import (
	"context"

	"inkmuse/backend/internal/domain/asset"
	"inkmuse/backend/internal/domain/chapter"
	"inkmuse/backend/internal/domain/conversation"
	"inkmuse/backend/internal/domain/generation"
	"inkmuse/backend/internal/domain/llmprovider"
	"inkmuse/backend/internal/domain/metric"
	"inkmuse/backend/internal/domain/project"
	"inkmuse/backend/internal/domain/prompt"
)

// ReadinessChecker reports whether the storage backend is ready.
type ReadinessChecker interface {
	CheckReadiness(ctx context.Context) error
}

// TxRunner executes function closures within an optional storage transaction.
type TxRunner interface {
	InTx(ctx context.Context, fn func(context.Context) error) error
}

// Repositories 将所有领域存储库(repository)契约组合在一起，用于运行时装配(runtime wiring)。
type Repositories struct {
	Projects          project.ProjectRepository
	Assets            asset.AssetRepository
	Chapters          chapter.ChapterRepository
	Conversations     conversation.ConversationRepository
	GenerationRecords generation.GenerationRecordRepository
	MetricEvents      metric.MetricEventRepository
	PromptOverrides   prompt.OverrideRepository
	LLMProviders      llmprovider.Repository
	TxRunner          TxRunner

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
