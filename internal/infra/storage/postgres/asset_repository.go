package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	assetdomain "novelforge/backend/internal/domain/asset"
)

// AssetRepository 在 PostgreSQL 中持久化资产(asset)。
type AssetRepository struct {
	db *sql.DB
}

// NewAssetRepository 创建 PostgreSQL 资产(asset)存储库。
func NewAssetRepository(db *sql.DB) *AssetRepository {
	return &AssetRepository{db: db}
}

func (r *AssetRepository) Create(ctx context.Context, entity *assetdomain.Asset) error {
	if entity == nil {
		return fmt.Errorf("asset must not be nil")
	}
	if err := entity.Validate(); err != nil {
		return err
	}

	executor := executorFromContext(ctx, r.db)
	_, err := executor.ExecContext(ctx, `
		INSERT INTO assets (id, project_id, type, title, content, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, entity.ID, entity.ProjectID, entity.Type, entity.Title, entity.Content, entity.CreatedAt, entity.UpdatedAt)
	return mapExecError(err)
}

func (r *AssetRepository) GetByID(ctx context.Context, id string) (*assetdomain.Asset, error) {
	entity := &assetdomain.Asset{}
	executor := executorFromContext(ctx, r.db)
	err := executor.QueryRowContext(ctx, `
		SELECT id, project_id, type, title, content, created_at, updated_at
		FROM assets
		WHERE id = $1
	`, id).Scan(
		&entity.ID,
		&entity.ProjectID,
		&entity.Type,
		&entity.Title,
		&entity.Content,
		&entity.CreatedAt,
		&entity.UpdatedAt,
	)
	if err != nil {
		return nil, mapQueryError(err)
	}
	return entity, nil
}

func (r *AssetRepository) ListByProject(ctx context.Context, params assetdomain.ListByProjectParams) ([]*assetdomain.Asset, error) {
	query := `
		SELECT id, project_id, type, title, content, created_at, updated_at
		FROM assets
		WHERE project_id = $1
		ORDER BY created_at ASC, id ASC
	`
	args := []any{params.ProjectID}
	query, args = appendPagination(query, params.Limit, params.Offset, args)
	return r.list(ctx, query, args...)
}

func (r *AssetRepository) ListByProjectAndType(ctx context.Context, params assetdomain.ListByProjectAndTypeParams) ([]*assetdomain.Asset, error) {
	query := `
		SELECT id, project_id, type, title, content, created_at, updated_at
		FROM assets
		WHERE project_id = $1 AND type = $2
		ORDER BY created_at ASC, id ASC
	`
	args := []any{params.ProjectID, params.Type}
	query, args = appendPagination(query, params.Limit, params.Offset, args)
	return r.list(ctx, query, args...)
}

func (r *AssetRepository) Update(ctx context.Context, entity *assetdomain.Asset) error {
	if entity == nil {
		return fmt.Errorf("asset must not be nil")
	}
	if err := entity.Validate(); err != nil {
		return err
	}

	executor := executorFromContext(ctx, r.db)
	result, err := executor.ExecContext(ctx, `
		UPDATE assets
		SET type = $2, title = $3, content = $4, updated_at = $5
		WHERE id = $1
	`, entity.ID, entity.Type, entity.Title, entity.Content, entity.UpdatedAt)
	if err != nil {
		return mapExecError(err)
	}
	return ensureRowsAffected(result)
}

func (r *AssetRepository) UpdateIfUnchanged(ctx context.Context, entity *assetdomain.Asset, expectedUpdatedAt time.Time) (bool, error) {
	if entity == nil {
		return false, fmt.Errorf("asset must not be nil")
	}
	if err := entity.Validate(); err != nil {
		return false, err
	}
	if expectedUpdatedAt.IsZero() {
		return false, fmt.Errorf("expected_updated_at must not be zero")
	}

	executor := executorFromContext(ctx, r.db)
	result, err := executor.ExecContext(ctx, `
		UPDATE assets
		SET type = $2, title = $3, content = $4, updated_at = $5
		WHERE id = $1 AND updated_at = $6
	`, entity.ID, entity.Type, entity.Title, entity.Content, entity.UpdatedAt, expectedUpdatedAt)
	if err != nil {
		return false, mapExecError(err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected > 0, nil
}

func (r *AssetRepository) Delete(ctx context.Context, id string) error {
	executor := executorFromContext(ctx, r.db)
	result, err := executor.ExecContext(ctx, `DELETE FROM assets WHERE id = $1`, id)
	if err != nil {
		return mapExecError(err)
	}
	return ensureRowsAffected(result)
}

func (r *AssetRepository) list(ctx context.Context, query string, args ...any) ([]*assetdomain.Asset, error) {
	executor := executorFromContext(ctx, r.db)
	rows, err := executor.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*assetdomain.Asset, 0)
	for rows.Next() {
		entity := &assetdomain.Asset{}
		if err := rows.Scan(
			&entity.ID,
			&entity.ProjectID,
			&entity.Type,
			&entity.Title,
			&entity.Content,
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
