package chapter

import (
	"context"
	"time"
)

// ListByProjectParams 定义项目下的章节(chapter)过滤器。
type ListByProjectParams struct {
	ProjectID string
	Limit     int
	Offset    int
}

// ChapterRepository 定义章节(chapter)持久化行为。
type ChapterRepository interface {
	Create(ctx context.Context, chapter *Chapter) error
	GetByID(ctx context.Context, id string) (*Chapter, error)
	ListByProject(ctx context.Context, params ListByProjectParams) ([]*Chapter, error)
	Update(ctx context.Context, chapter *Chapter) error
	// UpdateIfUnchanged 使用 optimistic locking 避免并发请求覆盖新稿状态。
	UpdateIfUnchanged(ctx context.Context, chapter *Chapter, expectedUpdatedAt time.Time) (bool, error)
}
