package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"inkmuse/backend/internal/domain/project"
)

// ProjectRepository 在内存中存储项目(project)。
type ProjectRepository struct {
	mu    sync.RWMutex
	items map[string]*project.Project
	order []string
}

// NewProjectRepository 创建内存项目(project)存储库。
func NewProjectRepository() *ProjectRepository {
	return &ProjectRepository{
		items: make(map[string]*project.Project),
	}
}

func (r *ProjectRepository) Create(_ context.Context, entity *project.Project) error {
	if entity == nil {
		return fmt.Errorf("project must not be nil")
	}
	if err := entity.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.items[entity.ID]; exists {
		return ErrAlreadyExists
	}

	r.items[entity.ID] = cloneProject(entity)
	r.order = append(r.order, entity.ID)
	return nil
}

func (r *ProjectRepository) GetByID(_ context.Context, id string) (*project.Project, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entity, exists := r.items[id]
	if !exists {
		return nil, ErrNotFound
	}
	return cloneProject(entity), nil
}

func (r *ProjectRepository) List(_ context.Context, params project.ListParams) ([]*project.Project, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*project.Project, 0, len(r.order))
	for _, id := range r.order {
		entity := r.items[id]
		if params.Status != "" && entity.Status != params.Status {
			continue
		}
		result = append(result, cloneProject(entity))
	}

	start, end := sliceBounds(params.Limit, params.Offset, len(result))
	return result[start:end], nil
}

func (r *ProjectRepository) Update(_ context.Context, entity *project.Project) error {
	if entity == nil {
		return fmt.Errorf("project must not be nil")
	}
	if err := entity.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.items[entity.ID]; !exists {
		return ErrNotFound
	}

	r.items[entity.ID] = cloneProject(entity)
	return nil
}

func (r *ProjectRepository) UpdateIfUnchanged(_ context.Context, entity *project.Project, expectedUpdatedAt time.Time) (bool, error) {
	if entity == nil {
		return false, fmt.Errorf("project must not be nil")
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

	r.items[entity.ID] = cloneProject(entity)
	return true, nil
}
