package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	projectdomain "inkmuse/backend/internal/domain/project"
)

// ProjectRepository 在 PostgreSQL 中持久化项目(project)。
type ProjectRepository struct {
	db *sql.DB
}

// NewProjectRepository 创建 PostgreSQL 项目(project)存储库。
func NewProjectRepository(db *sql.DB) *ProjectRepository {
	return &ProjectRepository{db: db}
}

func (r *ProjectRepository) Create(ctx context.Context, entity *projectdomain.Project) error {
	if entity == nil {
		return fmt.Errorf("project must not be nil")
	}
	if err := entity.Validate(); err != nil {
		return err
	}

	executor := executorFromContext(ctx, r.db)
	_, err := executor.ExecContext(ctx, `
		INSERT INTO projects (id, title, summary, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, entity.ID, entity.Title, entity.Summary, entity.Status, entity.CreatedAt, entity.UpdatedAt)
	return mapExecError(err)
}

func (r *ProjectRepository) GetByID(ctx context.Context, id string) (*projectdomain.Project, error) {
	entity := &projectdomain.Project{}
	executor := executorFromContext(ctx, r.db)
	err := executor.QueryRowContext(ctx, `
		SELECT id, title, summary, status, created_at, updated_at
		FROM projects
		WHERE id = $1
	`, id).Scan(
		&entity.ID,
		&entity.Title,
		&entity.Summary,
		&entity.Status,
		&entity.CreatedAt,
		&entity.UpdatedAt,
	)
	if err != nil {
		return nil, mapQueryError(err)
	}
	return entity, nil
}

func (r *ProjectRepository) List(ctx context.Context, params projectdomain.ListParams) ([]*projectdomain.Project, error) {
	query := `
		SELECT id, title, summary, status, created_at, updated_at
		FROM projects
	`
	args := make([]any, 0, 3)
	if params.Status != "" {
		query += ` WHERE status = $1`
		args = append(args, params.Status)
	}
	query += ` ORDER BY created_at ASC, id ASC`
	query, args = appendPagination(query, params.Limit, params.Offset, args)

	executor := executorFromContext(ctx, r.db)
	rows, err := executor.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*projectdomain.Project, 0)
	for rows.Next() {
		entity := &projectdomain.Project{}
		if err := rows.Scan(
			&entity.ID,
			&entity.Title,
			&entity.Summary,
			&entity.Status,
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

func (r *ProjectRepository) Update(ctx context.Context, entity *projectdomain.Project) error {
	if entity == nil {
		return fmt.Errorf("project must not be nil")
	}
	if err := entity.Validate(); err != nil {
		return err
	}

	executor := executorFromContext(ctx, r.db)
	result, err := executor.ExecContext(ctx, `
		UPDATE projects
		SET title = $2, summary = $3, status = $4, updated_at = $5
		WHERE id = $1
	`, entity.ID, entity.Title, entity.Summary, entity.Status, entity.UpdatedAt)
	if err != nil {
		return mapExecError(err)
	}
	return ensureRowsAffected(result)
}

func (r *ProjectRepository) UpdateIfUnchanged(ctx context.Context, entity *projectdomain.Project, expectedUpdatedAt time.Time) (bool, error) {
	if entity == nil {
		return false, fmt.Errorf("project must not be nil")
	}
	if err := entity.Validate(); err != nil {
		return false, err
	}
	if expectedUpdatedAt.IsZero() {
		return false, fmt.Errorf("expected_updated_at must not be zero")
	}

	executor := executorFromContext(ctx, r.db)
	result, err := executor.ExecContext(ctx, `
		UPDATE projects
		SET title = $2, summary = $3, status = $4, updated_at = $5
		WHERE id = $1 AND updated_at = $6
	`, entity.ID, entity.Title, entity.Summary, entity.Status, entity.UpdatedAt, expectedUpdatedAt)
	if err != nil {
		return false, mapExecError(err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected > 0, nil
}
