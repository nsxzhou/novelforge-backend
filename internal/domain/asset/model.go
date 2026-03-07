package asset

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	TypeWorldbuilding = "worldbuilding"
	TypeCharacter     = "character"
	TypeOutline       = "outline"
)

// Asset 存储结构化或自由形式的项目上下文。
type Asset struct {
	ID        string
	ProjectID string
	Type      string
	Title     string
	Content   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Validate 以快速失败(fail-fast)的方式验证资产(asset)字段。
func (a Asset) Validate() error {
	if _, err := uuid.Parse(a.ID); err != nil {
		return fmt.Errorf("id must be a valid UUID")
	}
	if _, err := uuid.Parse(a.ProjectID); err != nil {
		return fmt.Errorf("project_id must be a valid UUID")
	}
	if !IsValidType(a.Type) {
		return fmt.Errorf("type must be one of worldbuilding, character, outline")
	}
	if strings.TrimSpace(a.Title) == "" {
		return fmt.Errorf("title must not be empty")
	}
	if strings.TrimSpace(a.Content) == "" {
		return fmt.Errorf("content must not be empty")
	}
	if a.CreatedAt.IsZero() {
		return fmt.Errorf("created_at must not be zero")
	}
	if a.UpdatedAt.IsZero() {
		return fmt.Errorf("updated_at must not be zero")
	}
	if a.UpdatedAt.Before(a.CreatedAt) {
		return fmt.Errorf("updated_at must not be before created_at")
	}
	return nil
}

// IsValidType 报告该资产(asset)类型是否受支持。
func IsValidType(assetType string) bool {
	switch assetType {
	case TypeWorldbuilding, TypeCharacter, TypeOutline:
		return true
	default:
		return false
	}
}
