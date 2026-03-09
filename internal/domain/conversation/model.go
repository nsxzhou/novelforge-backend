package conversation

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	TargetTypeProject = "project"
	TargetTypeAsset   = "asset"

	MessageRoleSystem    = "system"
	MessageRoleUser      = "user"
	MessageRoleAssistant = "assistant"
)

// Message 存储一轮对话。
type Message struct {
	ID        string
	Role      string
	Content   string
	CreatedAt time.Time
}

// Validate 验证单条消息。
func (m Message) Validate() error {
	if _, err := uuid.Parse(m.ID); err != nil {
		return fmt.Errorf("id must be a valid UUID")
	}
	if !IsValidMessageRole(m.Role) {
		return fmt.Errorf("role must be one of system, user, assistant")
	}
	if strings.TrimSpace(m.Content) == "" {
		return fmt.Errorf("content must not be empty")
	}
	if m.CreatedAt.IsZero() {
		return fmt.Errorf("created_at must not be zero")
	}
	return nil
}

// PendingSuggestion 存储最新待确认的建议草案。
type PendingSuggestion struct {
	Title   string
	Summary string
	Content string
}

// Validate 验证待确认建议字段。
func (s PendingSuggestion) Validate(targetType string) error {
	normalized := s.normalized()
	if normalized.Title == "" {
		return fmt.Errorf("title must not be empty")
	}

	switch targetType {
	case TargetTypeProject:
		if normalized.Summary == "" {
			return fmt.Errorf("summary must not be empty")
		}
		if normalized.Content != "" {
			return fmt.Errorf("content must be empty for project suggestion")
		}
	case TargetTypeAsset:
		if normalized.Content == "" {
			return fmt.Errorf("content must not be empty")
		}
		if normalized.Summary != "" {
			return fmt.Errorf("summary must be empty for asset suggestion")
		}
	default:
		return fmt.Errorf("target_type must be one of project, asset")
	}

	return nil
}

func (s PendingSuggestion) normalized() PendingSuggestion {
	return PendingSuggestion{
		Title:   strings.TrimSpace(s.Title),
		Summary: strings.TrimSpace(s.Summary),
		Content: strings.TrimSpace(s.Content),
	}
}

// Conversation 存储消息历史记录、目标链接和最新待确认建议。
type Conversation struct {
	ID                string
	ProjectID         string
	TargetType        string
	TargetID          string
	Messages          []Message
	PendingSuggestion *PendingSuggestion
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Validate 以快速失败(fail-fast)的方式验证对话(conversation)字段。
func (c Conversation) Validate() error {
	if _, err := uuid.Parse(c.ID); err != nil {
		return fmt.Errorf("id must be a valid UUID")
	}
	if _, err := uuid.Parse(c.ProjectID); err != nil {
		return fmt.Errorf("project_id must be a valid UUID")
	}
	if !IsValidTargetType(c.TargetType) {
		return fmt.Errorf("target_type must be one of project, asset")
	}
	if _, err := uuid.Parse(c.TargetID); err != nil {
		return fmt.Errorf("target_id must be a valid UUID")
	}
	for i := range c.Messages {
		if err := c.Messages[i].Validate(); err != nil {
			return fmt.Errorf("invalid message: %w", err)
		}
	}
	if c.PendingSuggestion != nil {
		if err := c.PendingSuggestion.Validate(c.TargetType); err != nil {
			return fmt.Errorf("invalid pending_suggestion: %w", err)
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

// AppendMessage 将经过验证的消息附加到对话的末尾。
func (c *Conversation) AppendMessage(message Message) error {
	if err := message.Validate(); err != nil {
		return err
	}
	c.Messages = append(c.Messages, message)
	if message.CreatedAt.After(c.UpdatedAt) {
		c.UpdatedAt = message.CreatedAt
	}
	return nil
}

// ReplacePendingSuggestion 更新最新待确认建议。
func (c *Conversation) ReplacePendingSuggestion(suggestion PendingSuggestion, updatedAt time.Time) error {
	normalized := suggestion.normalized()
	if err := normalized.Validate(c.TargetType); err != nil {
		return err
	}
	c.PendingSuggestion = &normalized
	if updatedAt.After(c.UpdatedAt) {
		c.UpdatedAt = updatedAt
	}
	return nil
}

// ClearPendingSuggestion 清空最新待确认建议。
func (c *Conversation) ClearPendingSuggestion(updatedAt time.Time) {
	c.PendingSuggestion = nil
	if updatedAt.After(c.UpdatedAt) {
		c.UpdatedAt = updatedAt
	}
}

// IsValidTargetType 报告该对话所针对的目标类型是否受支持。
func IsValidTargetType(targetType string) bool {
	switch targetType {
	case TargetTypeProject, TargetTypeAsset:
		return true
	default:
		return false
	}
}

// IsValidMessageRole 报告该对话消息的角色是否受支持。
func IsValidMessageRole(role string) bool {
	switch role {
	case MessageRoleSystem, MessageRoleUser, MessageRoleAssistant:
		return true
	default:
		return false
	}
}

