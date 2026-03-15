package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	promptdomain "inkmuse/backend/internal/domain/prompt"
)

// PromptOverrideRepository 在内存中存储项目级 prompt 覆盖。
type PromptOverrideRepository struct {
	mu    sync.RWMutex
	items map[string]*promptdomain.ProjectPromptOverride // key: "projectID:capability"
}

// NewPromptOverrideRepository 创建内存 prompt 覆盖存储库。
func NewPromptOverrideRepository() *PromptOverrideRepository {
	return &PromptOverrideRepository{
		items: make(map[string]*promptdomain.ProjectPromptOverride),
	}
}

func overrideKey(projectID, capability string) string {
	return projectID + ":" + capability
}

func (r *PromptOverrideRepository) ListByProject(_ context.Context, projectID string) ([]*promptdomain.ProjectPromptOverride, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*promptdomain.ProjectPromptOverride, 0)
	for _, item := range r.items {
		if item.ProjectID == projectID {
			result = append(result, clonePromptOverride(item))
		}
	}
	return result, nil
}

func (r *PromptOverrideRepository) GetByProjectAndCapability(_ context.Context, projectID string, capability string) (*promptdomain.ProjectPromptOverride, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	item, exists := r.items[overrideKey(projectID, capability)]
	if !exists {
		return nil, ErrNotFound
	}
	return clonePromptOverride(item), nil
}

func (r *PromptOverrideRepository) Upsert(_ context.Context, override *promptdomain.ProjectPromptOverride) error {
	if override == nil {
		return fmt.Errorf("override must not be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	key := overrideKey(override.ProjectID, override.Capability)
	now := time.Now().UTC()
	override.UpdatedAt = now

	existing, exists := r.items[key]
	if exists {
		override.CreatedAt = existing.CreatedAt
	} else {
		override.CreatedAt = now
	}

	r.items[key] = clonePromptOverride(override)
	return nil
}

func (r *PromptOverrideRepository) Delete(_ context.Context, projectID string, capability string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := overrideKey(projectID, capability)
	if _, exists := r.items[key]; !exists {
		return ErrNotFound
	}
	delete(r.items, key)
	return nil
}

func clonePromptOverride(src *promptdomain.ProjectPromptOverride) *promptdomain.ProjectPromptOverride {
	if src == nil {
		return nil
	}
	dst := *src
	return &dst
}
