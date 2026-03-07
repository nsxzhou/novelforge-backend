package chapter

import (
	"context"

	chapterdomain "novelforge/backend/internal/domain/chapter"
)

// Dependencies 声明章节(chapter)用例所需的领域依赖项。
type Dependencies struct {
	Chapters chapterdomain.ChapterRepository
}

// UseCase 定义章节(chapter)的应用边界。
type UseCase interface {
	Create(ctx context.Context, chapter *chapterdomain.Chapter) error
	GetByID(ctx context.Context, id string) (*chapterdomain.Chapter, error)
	ListByProject(ctx context.Context, params chapterdomain.ListByProjectParams) ([]*chapterdomain.Chapter, error)
	Update(ctx context.Context, chapter *chapterdomain.Chapter) error
}
