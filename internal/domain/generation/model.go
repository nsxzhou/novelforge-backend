package generation

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	KindAssetGeneration     = "asset_generation"
	KindChapterGeneration   = "chapter_generation"
	KindChapterContinuation = "chapter_continuation"
	KindChapterRewrite      = "chapter_rewrite"

	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
)

// GenerationRecord 存储一次生成任务的结果。
type GenerationRecord struct {
	ID               string
	ProjectID        string
	ChapterID        string
	ConversationID   string
	Kind             string
	Status           string
	InputSnapshotRef string
	OutputRef        string
	TokenUsage       int
	DurationMillis   int64
	ErrorMessage     string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// Validate 以快速失败(fail-fast)的方式验证生成记录字段。
func (r GenerationRecord) Validate() error {
	if _, err := uuid.Parse(r.ID); err != nil {
		return fmt.Errorf("id must be a valid UUID")
	}
	if _, err := uuid.Parse(r.ProjectID); err != nil {
		return fmt.Errorf("project_id must be a valid UUID")
	}
	if r.ChapterID != "" {
		if _, err := uuid.Parse(r.ChapterID); err != nil {
			return fmt.Errorf("chapter_id must be a valid UUID")
		}
	}
	if r.ConversationID != "" {
		if _, err := uuid.Parse(r.ConversationID); err != nil {
			return fmt.Errorf("conversation_id must be a valid UUID")
		}
	}
	if !IsValidKind(r.Kind) {
		return fmt.Errorf("kind must be one of asset_generation, chapter_generation, chapter_continuation, chapter_rewrite")
	}
	if !IsValidStatus(r.Status) {
		return fmt.Errorf("status must be one of pending, running, succeeded, failed")
	}
	if r.TokenUsage < 0 {
		return fmt.Errorf("token_usage must be greater than or equal to 0")
	}
	if r.DurationMillis < 0 {
		return fmt.Errorf("duration_millis must be greater than or equal to 0")
	}
	if r.CreatedAt.IsZero() {
		return fmt.Errorf("created_at must not be zero")
	}
	if r.UpdatedAt.IsZero() {
		return fmt.Errorf("updated_at must not be zero")
	}
	if r.UpdatedAt.Before(r.CreatedAt) {
		return fmt.Errorf("updated_at must not be before created_at")
	}
	return nil
}

// IsValidKind 报告该生成种类(kind)是否受支持。
func IsValidKind(kind string) bool {
	switch kind {
	case KindAssetGeneration, KindChapterGeneration, KindChapterContinuation, KindChapterRewrite:
		return true
	default:
		return false
	}
}

// IsValidStatus 报告该生成状态(status)是否受支持。
func IsValidStatus(status string) bool {
	switch status {
	case StatusPending, StatusRunning, StatusSucceeded, StatusFailed:
		return true
	default:
		return false
	}
}
