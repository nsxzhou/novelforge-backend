package postgres

import (
	"context"
	"database/sql"
	"fmt"

	chapterdomain "novelforge/backend/internal/domain/chapter"
)

// ChapterRepository 在 PostgreSQL 中持久化章节(chapter)。
type ChapterRepository struct {
	db *sql.DB
}

// NewChapterRepository 创建 PostgreSQL 章节(chapter)存储库。
func NewChapterRepository(db *sql.DB) *ChapterRepository {
	return &ChapterRepository{db: db}
}

func (r *ChapterRepository) Create(ctx context.Context, entity *chapterdomain.Chapter) error {
	if entity == nil {
		return fmt.Errorf("chapter must not be nil")
	}
	if err := entity.Validate(); err != nil {
		return err
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO chapters (
			id, project_id, title, ordinal, status, content,
			current_draft_id, current_draft_confirmed_at, current_draft_confirmed_by,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`,
		entity.ID,
		entity.ProjectID,
		entity.Title,
		entity.Ordinal,
		entity.Status,
		entity.Content,
		toNullString(entity.CurrentDraftID),
		toNullTime(entity.CurrentDraftConfirmedAt),
		toNullString(entity.CurrentDraftConfirmedBy),
		entity.CreatedAt,
		entity.UpdatedAt,
	)
	return mapExecError(err)
}

func (r *ChapterRepository) GetByID(ctx context.Context, id string) (*chapterdomain.Chapter, error) {
	entity := &chapterdomain.Chapter{}
	var currentDraftID sql.NullString
	var confirmedAt sql.NullTime
	var confirmedBy sql.NullString
	if err := r.db.QueryRowContext(ctx, `
		SELECT id, project_id, title, ordinal, status, content,
			current_draft_id, current_draft_confirmed_at, current_draft_confirmed_by,
			created_at, updated_at
		FROM chapters
		WHERE id = $1
	`, id).Scan(
		&entity.ID,
		&entity.ProjectID,
		&entity.Title,
		&entity.Ordinal,
		&entity.Status,
		&entity.Content,
		&currentDraftID,
		&confirmedAt,
		&confirmedBy,
		&entity.CreatedAt,
		&entity.UpdatedAt,
	); err != nil {
		return nil, mapQueryError(err)
	}
	if currentDraftID.Valid {
		entity.CurrentDraftID = currentDraftID.String
	}
	if confirmedAt.Valid {
		confirmed := confirmedAt.Time
		entity.CurrentDraftConfirmedAt = &confirmed
	}
	if confirmedBy.Valid {
		entity.CurrentDraftConfirmedBy = confirmedBy.String
	}
	return entity, nil
}

func (r *ChapterRepository) ListByProject(ctx context.Context, params chapterdomain.ListByProjectParams) ([]*chapterdomain.Chapter, error) {
	query := `
		SELECT id, project_id, title, ordinal, status, content,
			current_draft_id, current_draft_confirmed_at, current_draft_confirmed_by,
			created_at, updated_at
		FROM chapters
		WHERE project_id = $1
		ORDER BY ordinal ASC, created_at ASC, id ASC
	`
	args := []any{params.ProjectID}
	query, args = appendPagination(query, params.Limit, params.Offset, args)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*chapterdomain.Chapter, 0)
	for rows.Next() {
		entity := &chapterdomain.Chapter{}
		var currentDraftID sql.NullString
		var confirmedAt sql.NullTime
		var confirmedBy sql.NullString
		if err := rows.Scan(
			&entity.ID,
			&entity.ProjectID,
			&entity.Title,
			&entity.Ordinal,
			&entity.Status,
			&entity.Content,
			&currentDraftID,
			&confirmedAt,
			&confirmedBy,
			&entity.CreatedAt,
			&entity.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if currentDraftID.Valid {
			entity.CurrentDraftID = currentDraftID.String
		}
		if confirmedAt.Valid {
			confirmed := confirmedAt.Time
			entity.CurrentDraftConfirmedAt = &confirmed
		}
		if confirmedBy.Valid {
			entity.CurrentDraftConfirmedBy = confirmedBy.String
		}
		items = append(items, entity)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *ChapterRepository) Update(ctx context.Context, entity *chapterdomain.Chapter) error {
	if entity == nil {
		return fmt.Errorf("chapter must not be nil")
	}
	if err := entity.Validate(); err != nil {
		return err
	}

	result, err := r.db.ExecContext(ctx, `
		UPDATE chapters
		SET project_id = $2, title = $3, ordinal = $4, status = $5, content = $6,
			current_draft_id = $7, current_draft_confirmed_at = $8, current_draft_confirmed_by = $9,
			updated_at = $10
		WHERE id = $1
	`,
		entity.ID,
		entity.ProjectID,
		entity.Title,
		entity.Ordinal,
		entity.Status,
		entity.Content,
		toNullString(entity.CurrentDraftID),
		toNullTime(entity.CurrentDraftConfirmedAt),
		toNullString(entity.CurrentDraftConfirmedBy),
		entity.UpdatedAt,
	)
	if err != nil {
		return mapExecError(err)
	}
	return ensureRowsAffected(result)
}
