package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"novelforge/backend/internal/domain/asset"
)

// AssetRepository 在内存中存储资产(asset)。
type AssetRepository struct {
	mu    sync.RWMutex
	items map[string]*asset.Asset
	order []string
}

// NewAssetRepository 创建内存资产(asset)存储库。
func NewAssetRepository() *AssetRepository {
	return &AssetRepository{
		items: make(map[string]*asset.Asset),
	}
}

func (r *AssetRepository) Create(_ context.Context, entity *asset.Asset) error {
	if entity == nil {
		return fmt.Errorf("asset must not be nil")
	}
	if err := entity.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.items[entity.ID]; exists {
		return ErrAlreadyExists
	}

	r.items[entity.ID] = cloneAsset(entity)
	r.order = append(r.order, entity.ID)
	return nil
}

func (r *AssetRepository) GetByID(_ context.Context, id string) (*asset.Asset, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entity, exists := r.items[id]
	if !exists {
		return nil, ErrNotFound
	}
	return cloneAsset(entity), nil
}

func (r *AssetRepository) ListByProject(_ context.Context, params asset.ListByProjectParams) ([]*asset.Asset, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*asset.Asset, 0, len(r.order))
	for _, id := range r.order {
		entity := r.items[id]
		if entity.ProjectID != params.ProjectID {
			continue
		}
		result = append(result, cloneAsset(entity))
	}

	start, end := sliceBounds(params.Limit, params.Offset, len(result))
	return result[start:end], nil
}

func (r *AssetRepository) ListByProjectAndType(_ context.Context, params asset.ListByProjectAndTypeParams) ([]*asset.Asset, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*asset.Asset, 0, len(r.order))
	for _, id := range r.order {
		entity := r.items[id]
		if entity.ProjectID != params.ProjectID {
			continue
		}
		if params.Type != "" && entity.Type != params.Type {
			continue
		}
		result = append(result, cloneAsset(entity))
	}

	start, end := sliceBounds(params.Limit, params.Offset, len(result))
	return result[start:end], nil
}

func (r *AssetRepository) Update(_ context.Context, entity *asset.Asset) error {
	if entity == nil {
		return fmt.Errorf("asset must not be nil")
	}
	if err := entity.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.items[entity.ID]; !exists {
		return ErrNotFound
	}

	r.items[entity.ID] = cloneAsset(entity)
	return nil
}

func (r *AssetRepository) UpdateIfUnchanged(_ context.Context, entity *asset.Asset, expectedUpdatedAt time.Time) (bool, error) {
	if entity == nil {
		return false, fmt.Errorf("asset must not be nil")
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

	r.items[entity.ID] = cloneAsset(entity)
	return true, nil
}

func (r *AssetRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.items[id]; !exists {
		return ErrNotFound
	}

	delete(r.items, id)
	for i, itemID := range r.order {
		if itemID == id {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
		}
	}
	return nil
}
