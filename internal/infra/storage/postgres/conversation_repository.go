package postgres

import (
	"context"
	"database/sql"
	"fmt"

	conversationdomain "novelforge/backend/internal/domain/conversation"
)

// ConversationRepository 在 PostgreSQL 中持久化对话(conversation)。
type ConversationRepository struct {
	db *sql.DB
}

// NewConversationRepository 创建 PostgreSQL 对话(conversation)存储库。
func NewConversationRepository(db *sql.DB) *ConversationRepository {
	return &ConversationRepository{db: db}
}

func (r *ConversationRepository) Create(ctx context.Context, entity *conversationdomain.Conversation) error {
	if entity == nil {
		return fmt.Errorf("conversation must not be nil")
	}
	if err := entity.Validate(); err != nil {
		return err
	}

	messages, err := marshalJSON(entity.Messages)
	if err != nil {
		return err
	}
	pendingSuggestion, err := marshalJSON(entity.PendingSuggestion)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO conversations (id, project_id, target_type, target_id, messages, pending_suggestion, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6::jsonb, $7, $8)
	`, entity.ID, entity.ProjectID, entity.TargetType, entity.TargetID, messages, pendingSuggestion, entity.CreatedAt, entity.UpdatedAt)
	return mapExecError(err)
}

func (r *ConversationRepository) GetByID(ctx context.Context, id string) (*conversationdomain.Conversation, error) {
	entity := &conversationdomain.Conversation{}
	var rawMessages []byte
	var rawPendingSuggestion []byte
	if err := r.db.QueryRowContext(ctx, `
		SELECT id, project_id, target_type, target_id, messages, pending_suggestion, created_at, updated_at
		FROM conversations
		WHERE id = $1
	`, id).Scan(
		&entity.ID,
		&entity.ProjectID,
		&entity.TargetType,
		&entity.TargetID,
		&rawMessages,
		&rawPendingSuggestion,
		&entity.CreatedAt,
		&entity.UpdatedAt,
	); err != nil {
		return nil, mapQueryError(err)
	}
	if err := unmarshalJSON(rawMessages, &entity.Messages); err != nil {
		return nil, err
	}
	if err := unmarshalJSON(rawPendingSuggestion, &entity.PendingSuggestion); err != nil {
		return nil, err
	}
	return entity, nil
}

func (r *ConversationRepository) Update(ctx context.Context, entity *conversationdomain.Conversation) error {
	if entity == nil {
		return fmt.Errorf("conversation must not be nil")
	}
	if err := entity.Validate(); err != nil {
		return err
	}

	messages, err := marshalJSON(entity.Messages)
	if err != nil {
		return err
	}
	pendingSuggestion, err := marshalJSON(entity.PendingSuggestion)
	if err != nil {
		return err
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE conversations
		SET messages = $2::jsonb, pending_suggestion = $3::jsonb, updated_at = $4
		WHERE id = $1
	`, entity.ID, messages, pendingSuggestion, entity.UpdatedAt)
	if err != nil {
		return mapExecError(err)
	}
	return ensureRowsAffected(result)
}

func (r *ConversationRepository) AppendMessage(ctx context.Context, params conversationdomain.AppendMessageParams) error {
	if params.ConversationID == "" {
		return fmt.Errorf("conversation_id must not be empty")
	}
	if err := params.Message.Validate(); err != nil {
		return err
	}

	entity, err := r.GetByID(ctx, params.ConversationID)
	if err != nil {
		return err
	}
	if err := entity.AppendMessage(params.Message); err != nil {
		return err
	}

	messages, err := marshalJSON(entity.Messages)
	if err != nil {
		return err
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE conversations
		SET messages = $2::jsonb, updated_at = $3
		WHERE id = $1
	`, entity.ID, messages, entity.UpdatedAt)
	if err != nil {
		return mapExecError(err)
	}
	return ensureRowsAffected(result)
}

func (r *ConversationRepository) ListByProject(ctx context.Context, params conversationdomain.ListByProjectParams) ([]*conversationdomain.Conversation, error) {
	query := `
		SELECT id, project_id, target_type, target_id, messages, pending_suggestion, created_at, updated_at
		FROM conversations
		WHERE project_id = $1
		ORDER BY created_at ASC, id ASC
	`
	args := []any{params.ProjectID}
	query, args = appendPagination(query, params.Limit, params.Offset, args)
	return r.list(ctx, query, args...)
}

func (r *ConversationRepository) ListByTarget(ctx context.Context, params conversationdomain.ListByTargetParams) ([]*conversationdomain.Conversation, error) {
	query := `
		SELECT id, project_id, target_type, target_id, messages, pending_suggestion, created_at, updated_at
		FROM conversations
		WHERE project_id = $1 AND target_type = $2 AND target_id = $3
		ORDER BY created_at ASC, id ASC
	`
	args := []any{params.ProjectID, params.TargetType, params.TargetID}
	query, args = appendPagination(query, params.Limit, params.Offset, args)
	return r.list(ctx, query, args...)
}

func (r *ConversationRepository) list(ctx context.Context, query string, args ...any) ([]*conversationdomain.Conversation, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*conversationdomain.Conversation, 0)
	for rows.Next() {
		entity := &conversationdomain.Conversation{}
		var rawMessages []byte
		var rawPendingSuggestion []byte
		if err := rows.Scan(
			&entity.ID,
			&entity.ProjectID,
			&entity.TargetType,
			&entity.TargetID,
			&rawMessages,
			&rawPendingSuggestion,
			&entity.CreatedAt,
			&entity.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if err := unmarshalJSON(rawMessages, &entity.Messages); err != nil {
			return nil, err
		}
		if err := unmarshalJSON(rawPendingSuggestion, &entity.PendingSuggestion); err != nil {
			return nil, err
		}
		items = append(items, entity)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
