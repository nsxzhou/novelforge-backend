package postgres

import (
	"context"
	"database/sql"
	"fmt"

	metricdomain "inkmuse/backend/internal/domain/metric"
)

// MetricEventRepository 在 PostgreSQL 中持久化指标(metric)事件。
type MetricEventRepository struct {
	db *sql.DB
}

// NewMetricEventRepository 创建 PostgreSQL 指标(metric)事件存储库。
func NewMetricEventRepository(db *sql.DB) *MetricEventRepository {
	return &MetricEventRepository{db: db}
}

func (r *MetricEventRepository) Append(ctx context.Context, entity *metricdomain.MetricEvent) error {
	if entity == nil {
		return fmt.Errorf("metric event must not be nil")
	}
	if err := entity.Validate(); err != nil {
		return err
	}

	executor := executorFromContext(ctx, r.db)
	labels, err := marshalJSON(entity.Labels)
	if err != nil {
		return err
	}
	stats, err := marshalJSON(entity.Stats)
	if err != nil {
		return err
	}
	_, err = executor.ExecContext(ctx, `
		INSERT INTO metric_events (id, event_name, project_id, chapter_id, labels, stats, occurred_at)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6::jsonb, $7)
	`, entity.ID, entity.EventName, entity.ProjectID, toNullString(entity.ChapterID), labels, stats, entity.OccurredAt)
	return mapExecError(err)
}

func (r *MetricEventRepository) ListByProject(ctx context.Context, params metricdomain.ListByProjectParams) ([]*metricdomain.MetricEvent, error) {
	query := `
		SELECT id, event_name, project_id, chapter_id, labels, stats, occurred_at
		FROM metric_events
		WHERE project_id = $1
	`
	args := []any{params.ProjectID}
	if params.EventName != "" {
		query += fmt.Sprintf(" AND event_name = $%d", len(args)+1)
		args = append(args, params.EventName)
	}
	query += ` ORDER BY occurred_at ASC, id ASC`
	query, args = appendPagination(query, params.Limit, params.Offset, args)

	executor := executorFromContext(ctx, r.db)
	rows, err := executor.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*metricdomain.MetricEvent, 0)
	for rows.Next() {
		entity := &metricdomain.MetricEvent{}
		var chapterID sql.NullString
		var rawLabels []byte
		var rawStats []byte
		if err := rows.Scan(
			&entity.ID,
			&entity.EventName,
			&entity.ProjectID,
			&chapterID,
			&rawLabels,
			&rawStats,
			&entity.OccurredAt,
		); err != nil {
			return nil, err
		}
		if chapterID.Valid {
			entity.ChapterID = chapterID.String
		}
		if err := unmarshalJSON(rawLabels, &entity.Labels); err != nil {
			return nil, err
		}
		if err := unmarshalJSON(rawStats, &entity.Stats); err != nil {
			return nil, err
		}
		items = append(items, entity)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
