package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"novelforge/backend/internal/domain/chapter"
)

// ChapterRepository 在内存中存储章节(chapter)。
type ChapterRepository struct {
	mu    sync.RWMutex
	items map[string]*chapter.Chapter
	order []string
}

// NewChapterRepository 创建内存章节(chapter)存储库。
func NewChapterRepository() *ChapterRepository {
	return &ChapterRepository{
		items: make(map[string]*chapter.Chapter),
	}
}

func (r *ChapterRepository) Create(_ context.Context, entity *chapter.Chapter) error {
	if entity == nil {
		return fmt.Errorf("chapter must not be nil")
	}
	if err := entity.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.items[entity.ID]; exists {
		return ErrAlreadyExists
	}

	r.items[entity.ID] = cloneChapter(entity)
	r.order = append(r.order, entity.ID)
	return nil
}

func (r *ChapterRepository) GetByID(_ context.Context, id string) (*chapter.Chapter, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entity, exists := r.items[id]
	if !exists {
		return nil, ErrNotFound
	}
	return cloneChapter(entity), nil
}

func (r *ChapterRepository) ListByProject(_ context.Context, params chapter.ListByProjectParams) ([]*chapter.Chapter, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*chapter.Chapter, 0, len(r.order))
	for _, id := range r.order {
		entity := r.items[id]
		if entity.ProjectID != params.ProjectID {
			continue
		}
		result = append(result, cloneChapter(entity))
	}

	start, end := sliceBounds(params.Limit, params.Offset, len(result))
	return result[start:end], nil
}

func (r *ChapterRepository) Update(_ context.Context, entity *chapter.Chapter) error {
	if entity == nil {
		return fmt.Errorf("chapter must not be nil")
	}
	if err := entity.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.items[entity.ID]; !exists {
		return ErrNotFound
	}

	r.items[entity.ID] = cloneChapter(entity)
	return nil
}

func (r *ChapterRepository) UpdateIfUnchanged(_ context.Context, entity *chapter.Chapter, expectedUpdatedAt time.Time) (bool, error) {
	if entity == nil {
		return false, fmt.Errorf("chapter must not be nil")
	}
	if err := entity.Validate(); err != nil {
		return false, err
	}
	if expectedUpdatedAt.IsZero() {
		return false, fmt.Errorf("expected_updated_at must not be zero")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	current, exists := r.items[entity.ID]
	if !exists {
		return false, ErrNotFound
	}
	if !current.UpdatedAt.Equal(expectedUpdatedAt) {
		return false, nil
	}

	r.items[entity.ID] = cloneChapter(entity)
	return true, nil
}
