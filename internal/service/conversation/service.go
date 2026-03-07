package conversation

import (
	"context"

	conversationdomain "novelforge/backend/internal/domain/conversation"
)

// Dependencies 声明对话(conversation)用例所需的领域依赖项。
type Dependencies struct {
	Conversations conversationdomain.ConversationRepository
}

// UseCase 定义对话(conversation)的应用边界。
type UseCase interface {
	Create(ctx context.Context, conversation *conversationdomain.Conversation) error
	GetByID(ctx context.Context, id string) (*conversationdomain.Conversation, error)
	AppendMessage(ctx context.Context, params conversationdomain.AppendMessageParams) error
	ListByProject(ctx context.Context, params conversationdomain.ListByProjectParams) ([]*conversationdomain.Conversation, error)
	ListByTarget(ctx context.Context, params conversationdomain.ListByTargetParams) ([]*conversationdomain.Conversation, error)
}
