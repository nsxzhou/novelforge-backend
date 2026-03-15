package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"inkmuse/backend/internal/domain/llmprovider"
	"inkmuse/backend/internal/infra/storage/shared"
)

// LLMProviderRepository persists LLM provider configurations in PostgreSQL.
type LLMProviderRepository struct {
	db *sql.DB
}

// NewLLMProviderRepository creates a PostgreSQL LLM provider repository.
func NewLLMProviderRepository(db *sql.DB) *LLMProviderRepository {
	return &LLMProviderRepository{db: db}
}

func (r *LLMProviderRepository) List(ctx context.Context) ([]*llmprovider.LLMProvider, error) {
	executor := executorFromContext(ctx, r.db)
	rows, err := executor.QueryContext(ctx, `
		SELECT id, provider, model, base_url, api_key, timeout_sec, priority, enabled, created_at, updated_at
		FROM llm_providers
		ORDER BY priority ASC, id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*llmprovider.LLMProvider, 0)
	for rows.Next() {
		p := &llmprovider.LLMProvider{}
		if err := rows.Scan(
			&p.ID, &p.Provider, &p.Model, &p.BaseURL, &p.APIKey,
			&p.TimeoutSec, &p.Priority, &p.Enabled, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *LLMProviderRepository) GetByID(ctx context.Context, id string) (*llmprovider.LLMProvider, error) {
	p := &llmprovider.LLMProvider{}
	executor := executorFromContext(ctx, r.db)
	err := executor.QueryRowContext(ctx, `
		SELECT id, provider, model, base_url, api_key, timeout_sec, priority, enabled, created_at, updated_at
		FROM llm_providers
		WHERE id = $1
	`, id).Scan(
		&p.ID, &p.Provider, &p.Model, &p.BaseURL, &p.APIKey,
		&p.TimeoutSec, &p.Priority, &p.Enabled, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, mapQueryError(err)
	}
	return p, nil
}

func (r *LLMProviderRepository) Upsert(ctx context.Context, provider *llmprovider.LLMProvider) error {
	if provider == nil {
		return fmt.Errorf("provider must not be nil")
	}

	now := time.Now().UTC()
	provider.UpdatedAt = now

	executor := executorFromContext(ctx, r.db)
	err := executor.QueryRowContext(ctx, `
		INSERT INTO llm_providers (id, provider, model, base_url, api_key, timeout_sec, priority, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO UPDATE
		SET provider    = EXCLUDED.provider,
		    model       = EXCLUDED.model,
		    base_url    = EXCLUDED.base_url,
		    api_key     = EXCLUDED.api_key,
		    timeout_sec = EXCLUDED.timeout_sec,
		    priority    = EXCLUDED.priority,
		    enabled     = EXCLUDED.enabled,
		    updated_at  = EXCLUDED.updated_at
		RETURNING created_at, updated_at
	`, provider.ID, provider.Provider, provider.Model, provider.BaseURL, provider.APIKey,
		provider.TimeoutSec, provider.Priority, provider.Enabled, now, now).Scan(
		&provider.CreatedAt,
		&provider.UpdatedAt,
	)
	return err
}

func (r *LLMProviderRepository) Delete(ctx context.Context, id string) error {
	executor := executorFromContext(ctx, r.db)
	result, err := executor.ExecContext(ctx, `
		DELETE FROM llm_providers WHERE id = $1
	`, id)
	if err != nil {
		return mapExecError(err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return shared.ErrNotFound
	}
	return nil
}
