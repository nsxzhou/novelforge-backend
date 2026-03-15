package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	promptdomain "inkmuse/backend/internal/domain/prompt"
	"inkmuse/backend/internal/infra/storage/shared"
)

// PromptOverrideRepository 在 PostgreSQL 中持久化项目级 prompt 覆盖。
type PromptOverrideRepository struct {
	db *sql.DB
}

// NewPromptOverrideRepository 创建 PostgreSQL prompt 覆盖存储库。
func NewPromptOverrideRepository(db *sql.DB) *PromptOverrideRepository {
	return &PromptOverrideRepository{db: db}
}

func (r *PromptOverrideRepository) ListByProject(ctx context.Context, projectID string) ([]*promptdomain.ProjectPromptOverride, error) {
	executor := executorFromContext(ctx, r.db)
	rows, err := executor.QueryContext(ctx, `
		SELECT project_id, capability, system_tmpl, user_tmpl, created_at, updated_at
		FROM project_prompt_overrides
		WHERE project_id = $1
		ORDER BY capability ASC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*promptdomain.ProjectPromptOverride, 0)
	for rows.Next() {
		entity := &promptdomain.ProjectPromptOverride{}
		if err := rows.Scan(
			&entity.ProjectID,
			&entity.Capability,
			&entity.System,
			&entity.User,
			&entity.CreatedAt,
			&entity.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, entity)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *PromptOverrideRepository) GetByProjectAndCapability(ctx context.Context, projectID string, capability string) (*promptdomain.ProjectPromptOverride, error) {
	entity := &promptdomain.ProjectPromptOverride{}
	executor := executorFromContext(ctx, r.db)
	err := executor.QueryRowContext(ctx, `
		SELECT project_id, capability, system_tmpl, user_tmpl, created_at, updated_at
		FROM project_prompt_overrides
		WHERE project_id = $1 AND capability = $2
	`, projectID, capability).Scan(
		&entity.ProjectID,
		&entity.Capability,
		&entity.System,
		&entity.User,
		&entity.CreatedAt,
		&entity.UpdatedAt,
	)
	if err != nil {
		return nil, mapQueryError(err)
	}
	return entity, nil
}

func (r *PromptOverrideRepository) Upsert(ctx context.Context, override *promptdomain.ProjectPromptOverride) error {
	if override == nil {
		return fmt.Errorf("override must not be nil")
	}

	now := time.Now().UTC()
	override.UpdatedAt = now

	executor := executorFromContext(ctx, r.db)
	err := executor.QueryRowContext(ctx, `
		INSERT INTO project_prompt_overrides (project_id, capability, system_tmpl, user_tmpl, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (project_id, capability) DO UPDATE
		SET system_tmpl = EXCLUDED.system_tmpl,
		    user_tmpl   = EXCLUDED.user_tmpl,
		    updated_at  = EXCLUDED.updated_at
		RETURNING created_at, updated_at
	`, override.ProjectID, override.Capability, override.System, override.User, now, now).Scan(
		&override.CreatedAt,
		&override.UpdatedAt,
	)
	return err
}

func (r *PromptOverrideRepository) Delete(ctx context.Context, projectID string, capability string) error {
	executor := executorFromContext(ctx, r.db)
	result, err := executor.ExecContext(ctx, `
		DELETE FROM project_prompt_overrides
		WHERE project_id = $1 AND capability = $2
	`, projectID, capability)
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
