package conversation

import "context"

// AppendMessageParams 定义附加单条消息所需的输入参数。
type AppendMessageParams struct {
	ConversationID string
	Message        Message
}

// ListByProjectParams 定义项目下的对话(conversation)过滤器。
type ListByProjectParams struct {
	ProjectID string
	Limit     int
	Offset    int
}

// ListByTargetParams 定义特定目标的对话(conversation)过滤器。
type ListByTargetParams struct {
	ProjectID  string
	TargetType string
	TargetID   string
	Limit      int
	Offset     int
}

// ConversationRepository 定义对话(conversation)持久化行为。
type ConversationRepository interface {
	Create(ctx context.Context, conversation *Conversation) error
	GetByID(ctx context.Context, id string) (*Conversation, error)
	AppendMessage(ctx context.Context, params AppendMessageParams) error
	ListByProject(ctx context.Context, params ListByProjectParams) ([]*Conversation, error)
	ListByTarget(ctx context.Context, params ListByTargetParams) ([]*Conversation, error)
}
