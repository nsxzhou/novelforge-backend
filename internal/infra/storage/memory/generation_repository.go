package memory

import (
	"context"
	"fmt"
	"sync"

	"inkmuse/backend/internal/domain/generation"
)

// GenerationRecordRepository 在内存中存储生成记录。
type GenerationRecordRepository struct {
	mu    sync.RWMutex
	items map[string]*generation.GenerationRecord
	order []string
}

// NewGenerationRecordRepository 创建内存生成记录存储库。
func NewGenerationRecordRepository() *GenerationRecordRepository {
	return &GenerationRecordRepository{
		items: make(map[string]*generation.GenerationRecord),
	}
}

func (r *GenerationRecordRepository) Create(_ context.Context, entity *generation.GenerationRecord) error {
	if entity == nil {
		return fmt.Errorf("generation record must not be nil")
	}
	if err := entity.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.items[entity.ID]; exists {
		return ErrAlreadyExists
	}

	r.items[entity.ID] = cloneGenerationRecord(entity)
	r.order = append(r.order, entity.ID)
	return nil
}

func (r *GenerationRecordRepository) GetByID(_ context.Context, id string) (*generation.GenerationRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entity, exists := r.items[id]
	if !exists {
		return nil, ErrNotFound
	}
	return cloneGenerationRecord(entity), nil
}

func (r *GenerationRecordRepository) ListByProject(_ context.Context, params generation.ListByProjectParams) ([]*generation.GenerationRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*generation.GenerationRecord, 0, len(r.order))
	for _, id := range r.order {
		entity := r.items[id]
		if entity.ProjectID != params.ProjectID {
			continue
		}
		if params.Status != "" && entity.Status != params.Status {
			continue
		}
		result = append(result, cloneGenerationRecord(entity))
	}

	start, end := sliceBounds(params.Limit, params.Offset, len(result))
	return result[start:end], nil
}

func (r *GenerationRecordRepository) ListByChapter(_ context.Context, params generation.ListByChapterParams) ([]*generation.GenerationRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*generation.GenerationRecord, 0, len(r.order))
	for _, id := range r.order {
		entity := r.items[id]
		if entity.ChapterID != params.ChapterID {
			continue
		}
		if params.Status != "" && entity.Status != params.Status {
			continue
		}
		result = append(result, cloneGenerationRecord(entity))
	}

	start, end := sliceBounds(params.Limit, params.Offset, len(result))
	return result[start:end], nil
}

func (r *GenerationRecordRepository) UpdateStatus(_ context.Context, params generation.UpdateStatusParams) error {
	if params.ID == "" {
		return fmt.Errorf("id must not be empty")
	}
	if !generation.IsValidStatus(params.Status) {
		return fmt.Errorf("status must be one of pending, running, succeeded, failed")
	}
	if params.TokenUsage < 0 {
		return fmt.Errorf("token_usage must be greater than or equal to 0")
	}
	if params.DurationMillis < 0 {
		return fmt.Errorf("duration_millis must be greater than or equal to 0")
	}
	if params.UpdatedAt.IsZero() {
		return fmt.Errorf("updated_at must not be zero")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	entity, exists := r.items[params.ID]
	if !exists {
		return ErrNotFound
	}

	updated := cloneGenerationRecord(entity)
	updated.Status = params.Status
	updated.OutputRef = params.OutputRef
	updated.TokenUsage = params.TokenUsage
	updated.DurationMillis = params.DurationMillis
	updated.ErrorMessage = params.ErrorMessage
	updated.UpdatedAt = params.UpdatedAt
	if err := updated.Validate(); err != nil {
		return err
	}

	r.items[params.ID] = updated
	return nil
}
