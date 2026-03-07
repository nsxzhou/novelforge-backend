package conversation

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func validMessage(role, content string, createdAt time.Time) Message {
	return Message{
		ID:        uuid.NewString(),
		Role:      role,
		Content:   content,
		CreatedAt: createdAt,
	}
}

func validConversation() Conversation {
	now := time.Date(2026, 3, 6, 12, 0, 0, 0, time.UTC)
	return Conversation{
		ID:         uuid.NewString(),
		ProjectID:  uuid.NewString(),
		TargetType: TargetTypeProject,
		TargetID:   uuid.NewString(),
		Messages: []Message{
			validMessage(MessageRoleUser, "Start a new story", now),
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestConversationValidate(t *testing.T) {
	conversation := validConversation()
	if err := conversation.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestConversationValidateRejectsInvalidMessage(t *testing.T) {
	conversation := validConversation()
	conversation.Messages[0].Content = ""

	if err := conversation.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
}

func TestConversationAppendMessage(t *testing.T) {
	conversation := validConversation()
	message := validMessage(MessageRoleAssistant, "Here is a draft.", conversation.UpdatedAt.Add(time.Minute))

	if err := conversation.AppendMessage(message); err != nil {
		t.Fatalf("AppendMessage() error = %v", err)
	}
	if len(conversation.Messages) != 2 {
		t.Fatalf("len(Messages) = %d, want 2", len(conversation.Messages))
	}
	if conversation.Messages[1].Content != message.Content {
		t.Fatalf("Messages[1].Content = %q, want %q", conversation.Messages[1].Content, message.Content)
	}
	if !conversation.UpdatedAt.Equal(message.CreatedAt) {
		t.Fatalf("UpdatedAt = %v, want %v", conversation.UpdatedAt, message.CreatedAt)
	}
}
