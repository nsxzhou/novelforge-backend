package llmprovider

import "context"

// Repository defines persistence operations for LLM provider configurations.
type Repository interface {
	List(ctx context.Context) ([]*LLMProvider, error)
	GetByID(ctx context.Context, id string) (*LLMProvider, error)
	Upsert(ctx context.Context, provider *LLMProvider) error
	Delete(ctx context.Context, id string) error
}
