package memory

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"inkmuse/backend/internal/domain/llmprovider"
)

// LLMProviderRepository stores LLM provider configurations in memory.
type LLMProviderRepository struct {
	mu    sync.RWMutex
	items map[string]*llmprovider.LLMProvider
}

// NewLLMProviderRepository creates an in-memory LLM provider repository.
func NewLLMProviderRepository() *LLMProviderRepository {
	return &LLMProviderRepository{
		items: make(map[string]*llmprovider.LLMProvider),
	}
}

func (r *LLMProviderRepository) List(_ context.Context) ([]*llmprovider.LLMProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*llmprovider.LLMProvider, 0, len(r.items))
	for _, item := range r.items {
		result = append(result, cloneLLMProvider(item))
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Priority != result[j].Priority {
			return result[i].Priority < result[j].Priority
		}
		return result[i].ID < result[j].ID
	})
	return result, nil
}

func (r *LLMProviderRepository) GetByID(_ context.Context, id string) (*llmprovider.LLMProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	item, exists := r.items[id]
	if !exists {
		return nil, ErrNotFound
	}
	return cloneLLMProvider(item), nil
}

func (r *LLMProviderRepository) Upsert(_ context.Context, provider *llmprovider.LLMProvider) error {
	if provider == nil {
		return fmt.Errorf("provider must not be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	provider.UpdatedAt = now

	existing, exists := r.items[provider.ID]
	if exists {
		provider.CreatedAt = existing.CreatedAt
	} else {
		provider.CreatedAt = now
	}

	r.items[provider.ID] = cloneLLMProvider(provider)
	return nil
}

func (r *LLMProviderRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.items[id]; !exists {
		return ErrNotFound
	}
	delete(r.items, id)
	return nil
}

func cloneLLMProvider(src *llmprovider.LLMProvider) *llmprovider.LLMProvider {
	if src == nil {
		return nil
	}
	dst := *src
	return &dst
}
