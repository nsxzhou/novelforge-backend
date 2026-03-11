package postgres

import (
	"context"
	"database/sql"
	"fmt"

	generationdomain "novelforge/backend/internal/domain/generation"
)

// GenerationRecordRepository 在 PostgreSQL 中持久化生成记录。
type GenerationRecordRepository struct {
	db *sql.DB
}

// NewGenerationRecordRepository 创建 PostgreSQL 生成记录存储库。
func NewGenerationRecordRepository(db *sql.DB) *GenerationRecordRepository {
	return &GenerationRecordRepository{db: db}
}

func (r *GenerationRecordRepository) Create(ctx context.Context, entity *generationdomain.GenerationRecord) error {
	if entity == nil {
		return fmt.Errorf("generation record must not be nil")
	}
	if err := entity.Validate(); err != nil {
		return err
	}

	executor := executorFromContext(ctx, r.db)
	_, err := executor.ExecContext(ctx, `
		INSERT INTO generation_records (
			id, project_id, chapter_id, conversation_id, kind, status,
			input_snapshot_ref, output_ref, token_usage, duration_millis,
			error_message, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`,
		entity.ID,
		entity.ProjectID,
		toNullString(entity.ChapterID),
		toNullString(entity.ConversationID),
		entity.Kind,
		entity.Status,
		entity.InputSnapshotRef,
		entity.OutputRef,
		entity.TokenUsage,
		entity.DurationMillis,
		entity.ErrorMessage,
		entity.CreatedAt,
		entity.UpdatedAt,
	)
	return mapExecError(err)
}

func (r *GenerationRecordRepository) GetByID(ctx context.Context, id string) (*generationdomain.GenerationRecord, error) {
	entity := &generationdomain.GenerationRecord{}
	executor := executorFromContext(ctx, r.db)
	var chapterID sql.NullString
	var conversationID sql.NullString
	if err := executor.QueryRowContext(ctx, `
		SELECT id, project_id, chapter_id, conversation_id, kind, status,
			input_snapshot_ref, output_ref, token_usage, duration_millis,
			error_message, created_at, updated_at
		FROM generation_records
		WHERE id = $1
	`, id).Scan(
		&entity.ID,
		&entity.ProjectID,
		&chapterID,
		&conversationID,
		&entity.Kind,
		&entity.Status,
		&entity.InputSnapshotRef,
		&entity.OutputRef,
		&entity.TokenUsage,
		&entity.DurationMillis,
		&entity.ErrorMessage,
		&entity.CreatedAt,
		&entity.UpdatedAt,
	); err != nil {
		return nil, mapQueryError(err)
	}
	if chapterID.Valid {
		entity.ChapterID = chapterID.String
	}
	if conversationID.Valid {
		entity.ConversationID = conversationID.String
	}
	return entity, nil
}

func (r *GenerationRecordRepository) ListByProject(ctx context.Context, params generationdomain.ListByProjectParams) ([]*generationdomain.GenerationRecord, error) {
	query := `
		SELECT id, project_id, chapter_id, conversation_id, kind, status,
			input_snapshot_ref, output_ref, token_usage, duration_millis,
			error_message, created_at, updated_at
		FROM generation_records
		WHERE project_id = $1
	`
	args := []any{params.ProjectID}
	if params.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", len(args)+1)
		args = append(args, params.Status)
	}
	query += ` ORDER BY created_at ASC, id ASC`
	query, args = appendPagination(query, params.Limit, params.Offset, args)
	return r.list(ctx, query, args...)
}

func (r *GenerationRecordRepository) ListByChapter(ctx context.Context, params generationdomain.ListByChapterParams) ([]*generationdomain.GenerationRecord, error) {
	query := `
		SELECT id, project_id, chapter_id, conversation_id, kind, status,
			input_snapshot_ref, output_ref, token_usage, duration_millis,
			error_message, created_at, updated_at
		FROM generation_records
		WHERE chapter_id = $1
	`
	args := []any{params.ChapterID}
	if params.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", len(args)+1)
		args = append(args, params.Status)
	}
	query += ` ORDER BY created_at ASC, id ASC`
	query, args = appendPagination(query, params.Limit, params.Offset, args)
	return r.list(ctx, query, args...)
}

func (r *GenerationRecordRepository) UpdateStatus(ctx context.Context, params generationdomain.UpdateStatusParams) error {
	if params.ID == "" {
		return fmt.Errorf("id must not be empty")
	}
	if !generationdomain.IsValidStatus(params.Status) {
		return fmt.Errorf("status must be one of pending, running, succeeded, failed")
	}
	if params.TokenUsage < 0 {
		return fmt.Errorf("token_usage must be greater than or equal to 0")
	}
	if params.DurationMillis < 0 {
		return fmt.Errorf("duration_millis must be greater than or equal to 0")
	}
	if params.UpdatedAt.IsZero() {
		return fmt.Errorf("updated_at must not be zero")
	}

	executor := executorFromContext(ctx, r.db)
	result, err := executor.ExecContext(ctx, `
		UPDATE generation_records
		SET status = $2, output_ref = $3, token_usage = $4, duration_millis = $5,
			error_message = $6, updated_at = $7
		WHERE id = $1
	`, params.ID, params.Status, params.OutputRef, params.TokenUsage, params.DurationMillis, params.ErrorMessage, params.UpdatedAt)
	if err != nil {
		return mapExecError(err)
	}
	return ensureRowsAffected(result)
}

func (r *GenerationRecordRepository) list(ctx context.Context, query string, args ...any) ([]*generationdomain.GenerationRecord, error) {
	executor := executorFromContext(ctx, r.db)
	rows, err := executor.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*generationdomain.GenerationRecord, 0)
	for rows.Next() {
		entity := &generationdomain.GenerationRecord{}
		var chapterID sql.NullString
		var conversationID sql.NullString
		if err := rows.Scan(
			&entity.ID,
			&entity.ProjectID,
			&chapterID,
			&conversationID,
			&entity.Kind,
			&entity.Status,
			&entity.InputSnapshotRef,
			&entity.OutputRef,
			&entity.TokenUsage,
			&entity.DurationMillis,
			&entity.ErrorMessage,
			&entity.CreatedAt,
			&entity.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if chapterID.Valid {
			entity.ChapterID = chapterID.String
		}
		if conversationID.Valid {
			entity.ConversationID = conversationID.String
		}
		items = append(items, entity)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
