package metric

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	EventProjectCreated   = "project_created"
	EventAssetGenerated   = "asset_generated"
	EventChapterGenerated = "chapter_generated"
	EventChapterConfirmed = "chapter_confirmed"
	EventGenerationFailed = "generation_failed"
	// EventOperationCompleted 统一表示业务动作成功完成。
	EventOperationCompleted = "operation_completed"
	// EventOperationFailed 统一表示业务动作执行失败。
	EventOperationFailed = "operation_failed"
)

// MetricEvent 存储一个仅追加(append-only)的领域指标(metric)事件。
type MetricEvent struct {
	ID         string
	EventName  string
	ProjectID  string
	ChapterID  string
	Labels     map[string]string
	Stats      map[string]float64
	OccurredAt time.Time
}

// Validate 以快速失败(fail-fast)的方式验证指标(metric)事件字段。
func (e MetricEvent) Validate() error {
	if _, err := uuid.Parse(e.ID); err != nil {
		return fmt.Errorf("id must be a valid UUID")
	}
	if strings.TrimSpace(e.EventName) == "" {
		return fmt.Errorf("event_name must not be empty")
	}
	if _, err := uuid.Parse(e.ProjectID); err != nil {
		return fmt.Errorf("project_id must be a valid UUID")
	}
	if e.ChapterID != "" {
		if _, err := uuid.Parse(e.ChapterID); err != nil {
			return fmt.Errorf("chapter_id must be a valid UUID")
		}
	}
	if e.OccurredAt.IsZero() {
		return fmt.Errorf("occurred_at must not be zero")
	}
	return nil
}
