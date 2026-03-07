package generation

import (
	"context"

	generationdomain "novelforge/backend/internal/domain/generation"
)

// Dependencies 声明生成(generation)用例所需的领域依赖项。
type Dependencies struct {
	GenerationRecords generationdomain.GenerationRecordRepository
}

// UseCase 定义生成(generation)的应用边界。
type UseCase interface {
	Create(ctx context.Context, record *generationdomain.GenerationRecord) error
	GetByID(ctx context.Context, id string) (*generationdomain.GenerationRecord, error)
	ListByProject(ctx context.Context, params generationdomain.ListByProjectParams) ([]*generationdomain.GenerationRecord, error)
	ListByChapter(ctx context.Context, params generationdomain.ListByChapterParams) ([]*generationdomain.GenerationRecord, error)
	UpdateStatus(ctx context.Context, params generationdomain.UpdateStatusParams) error
}
