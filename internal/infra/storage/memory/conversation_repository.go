package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"novelforge/backend/internal/domain/conversation"
)

// ConversationRepository 在内存中存储对话(conversation)。
type ConversationRepository struct {
	mu    sync.RWMutex
	items map[string]*conversation.Conversation
	order []string
}

// NewConversationRepository 创建内存对话(conversation)存储库。
func NewConversationRepository() *ConversationRepository {
	return &ConversationRepository{
		items: make(map[string]*conversation.Conversation),
	}
}

func (r *ConversationRepository) Create(_ context.Context, entity *conversation.Conversation) error {
	if entity == nil {
		return fmt.Errorf("conversation must not be nil")
	}
	if err := entity.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.items[entity.ID]; exists {
		return ErrAlreadyExists
	}

	r.items[entity.ID] = cloneConversation(entity)
	r.order = append(r.order, entity.ID)
	return nil
}

func (r *ConversationRepository) GetByID(_ context.Context, id string) (*conversation.Conversation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entity, exists := r.items[id]
	if !exists {
		return nil, ErrNotFound
	}
	return cloneConversation(entity), nil
}

func (r *ConversationRepository) Update(_ context.Context, entity *conversation.Conversation) error {
	if entity == nil {
		return fmt.Errorf("conversation must not be nil")
	}
	if err := entity.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.items[entity.ID]; !exists {
		return ErrNotFound
	}
	r.items[entity.ID] = cloneConversation(entity)
	return nil
}

func (r *ConversationRepository) UpdateIfUnchanged(_ context.Context, entity *conversation.Conversation, expectedUpdatedAt time.Time) (bool, error) {
	if entity == nil {
		return false, fmt.Errorf("conversation must not be nil")
	}
	if err := entity.Validate(); err != nil {
		return false, err
	}
	if expectedUpdatedAt.IsZero() {
		return false, fmt.Errorf("expected_updated_at must not be zero")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	current, exists := r.items[entity.ID]
	if !exists {
		return false, ErrNotFound
	}
	if !current.UpdatedAt.Equal(expectedUpdatedAt) {
		return false, nil
	}

	r.items[entity.ID] = cloneConversation(entity)
	return true, nil
}

func (r *ConversationRepository) AppendMessage(_ context.Context, params conversation.AppendMessageParams) error {
	if params.ConversationID == "" {
		return fmt.Errorf("conversation_id must not be empty")
	}
	if err := params.Message.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	entity, exists := r.items[params.ConversationID]
	if !exists {
		return ErrNotFound
	}

	updated := cloneConversation(entity)
	if err := updated.AppendMessage(params.Message); err != nil {
		return err
	}
	r.items[params.ConversationID] = updated
	return nil
}

func (r *ConversationRepository) ListByProject(_ context.Context, params conversation.ListByProjectParams) ([]*conversation.Conversation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*conversation.Conversation, 0, len(r.order))
	for _, id := range r.order {
		entity := r.items[id]
		if entity.ProjectID != params.ProjectID {
			continue
		}
		result = append(result, cloneConversation(entity))
	}

	start, end := sliceBounds(params.Limit, params.Offset, len(result))
	return result[start:end], nil
}

func (r *ConversationRepository) ListByTarget(_ context.Context, params conversation.ListByTargetParams) ([]*conversation.Conversation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*conversation.Conversation, 0, len(r.order))
	for _, id := range r.order {
		entity := r.items[id]
		if entity.ProjectID != params.ProjectID {
			continue
		}
		if entity.TargetType != params.TargetType {
			continue
		}
		if entity.TargetID != params.TargetID {
			continue
		}
		result = append(result, cloneConversation(entity))
	}

	start, end := sliceBounds(params.Limit, params.Offset, len(result))
	return result[start:end], nil
}
