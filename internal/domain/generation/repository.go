package generation

import (
	"context"
	"time"
)

// ListByProjectParams 定义项目下的生成记录过滤器。
type ListByProjectParams struct {
	ProjectID string
	Limit     int
	Offset    int
	Status    string
}

// ListByChapterParams 定义章节下的生成记录过滤器。
type ListByChapterParams struct {
	ChapterID string
	Limit     int
	Offset    int
	Status    string
}

// UpdateStatusParams 定义生成状态更改时允许修改的字段。
type UpdateStatusParams struct {
	ID             string
	Status         string
	OutputRef      string
	TokenUsage     int
	DurationMillis int64
	ErrorMessage   string
	UpdatedAt      time.Time
}

// GenerationRecordRepository 定义生成记录持久化行为。
type GenerationRecordRepository interface {
	Create(ctx context.Context, record *GenerationRecord) error
	GetByID(ctx context.Context, id string) (*GenerationRecord, error)
	ListByProject(ctx context.Context, params ListByProjectParams) ([]*GenerationRecord, error)
	ListByChapter(ctx context.Context, params ListByChapterParams) ([]*GenerationRecord, error)
	UpdateStatus(ctx context.Context, params UpdateStatusParams) error
}
