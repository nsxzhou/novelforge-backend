package chapter

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	StatusDraft     = "draft"
	StatusConfirmed = "confirmed"
)

// Chapter 表示生成或编辑的章节草稿。
type Chapter struct {
	ID                      string
	ProjectID               string
	Title                   string
	Ordinal                 int
	Status                  string
	Content                 string
	CurrentDraftID          string
	CurrentDraftConfirmedAt *time.Time
	CurrentDraftConfirmedBy string
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

// Validate 以快速失败(fail-fast)的方式验证章节(chapter)字段。
func (c Chapter) Validate() error {
	if _, err := uuid.Parse(c.ID); err != nil {
		return fmt.Errorf("id must be a valid UUID")
	}
	if _, err := uuid.Parse(c.ProjectID); err != nil {
		return fmt.Errorf("project_id must be a valid UUID")
	}
	if strings.TrimSpace(c.Title) == "" {
		return fmt.Errorf("title must not be empty")
	}
	if c.Ordinal <= 0 {
		return fmt.Errorf("ordinal must be greater than 0")
	}
	if !IsValidStatus(c.Status) {
		return fmt.Errorf("status must be one of draft, confirmed")
	}
	if c.CurrentDraftID != "" {
		if _, err := uuid.Parse(c.CurrentDraftID); err != nil {
			return fmt.Errorf("current_draft_id must be a valid UUID")
		}
	}
	if c.CurrentDraftConfirmedAt != nil && c.CurrentDraftConfirmedBy == "" {
		return fmt.Errorf("current_draft_confirmed_by must not be empty when current_draft_confirmed_at is set")
	}
	if c.CurrentDraftConfirmedBy != "" {
		if _, err := uuid.Parse(c.CurrentDraftConfirmedBy); err != nil {
			return fmt.Errorf("current_draft_confirmed_by must be a valid UUID")
		}
		if c.CurrentDraftConfirmedAt == nil {
			return fmt.Errorf("current_draft_confirmed_at must not be nil when current_draft_confirmed_by is set")
		}
	}
	if c.Status == StatusConfirmed {
		if c.CurrentDraftID == "" {
			return fmt.Errorf("current_draft_id must not be empty when status is confirmed")
		}
		if c.CurrentDraftConfirmedAt == nil {
			return fmt.Errorf("current_draft_confirmed_at must not be nil when status is confirmed")
		}
		if c.CurrentDraftConfirmedBy == "" {
			return fmt.Errorf("current_draft_confirmed_by must not be empty when status is confirmed")
		}
	}
	if c.CreatedAt.IsZero() {
		return fmt.Errorf("created_at must not be zero")
	}
	if c.UpdatedAt.IsZero() {
		return fmt.Errorf("updated_at must not be zero")
	}
	if c.UpdatedAt.Before(c.CreatedAt) {
		return fmt.Errorf("updated_at must not be before created_at")
	}
	return nil
}

// IsValidStatus 报告该章节(chapter)状态是否受支持。
func IsValidStatus(status string) bool {
	switch status {
	case StatusDraft, StatusConfirmed:
		return true
	default:
		return false
	}
}
