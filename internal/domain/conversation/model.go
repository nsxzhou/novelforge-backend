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
	TargetTypeChapter = "chapter"

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

// Conversation 存储消息历史记录和目标链接。
type Conversation struct {
	ID         string
	ProjectID  string
	TargetType string
	TargetID   string
	Messages   []Message
	CreatedAt  time.Time
	UpdatedAt  time.Time
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
		return fmt.Errorf("target_type must be one of project, asset, chapter")
	}
	if _, err := uuid.Parse(c.TargetID); err != nil {
		return fmt.Errorf("target_id must be a valid UUID")
	}
	for i := range c.Messages {
		if err := c.Messages[i].Validate(); err != nil {
			return fmt.Errorf("invalid message: %w", err)
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

// IsValidTargetType 报告该对话所针对的目标类型是否受支持。
func IsValidTargetType(targetType string) bool {
	switch targetType {
	case TargetTypeProject, TargetTypeAsset, TargetTypeChapter:
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
