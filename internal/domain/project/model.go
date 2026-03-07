package project

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	StatusDraft    = "draft"
	StatusActive   = "active"
	StatusArchived = "archived"
)

// Project 是小说项目的根聚合(root aggregate)。
type Project struct {
	ID        string
	Title     string
	Summary   string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Validate 以快速失败(fail-fast)的方式验证项目(project)字段。
func (p Project) Validate() error {
	if _, err := uuid.Parse(p.ID); err != nil {
		return fmt.Errorf("id must be a valid UUID")
	}
	if strings.TrimSpace(p.Title) == "" {
		return fmt.Errorf("title must not be empty")
	}
	if strings.TrimSpace(p.Summary) == "" {
		return fmt.Errorf("summary must not be empty")
	}
	if !IsValidStatus(p.Status) {
		return fmt.Errorf("status must be one of draft, active, archived")
	}
	if p.CreatedAt.IsZero() {
		return fmt.Errorf("created_at must not be zero")
	}
	if p.UpdatedAt.IsZero() {
		return fmt.Errorf("updated_at must not be zero")
	}
	if p.UpdatedAt.Before(p.CreatedAt) {
		return fmt.Errorf("updated_at must not be before created_at")
	}
	return nil
}

// IsValidStatus 报告该项目(project)状态是否受支持。
func IsValidStatus(status string) bool {
	switch status {
	case StatusDraft, StatusActive, StatusArchived:
		return true
	default:
		return false
	}
}
