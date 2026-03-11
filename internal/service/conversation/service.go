package conversation

import (
	"context"

	assetdomain "novelforge/backend/internal/domain/asset"
	conversationdomain "novelforge/backend/internal/domain/conversation"
	projectdomain "novelforge/backend/internal/domain/project"
	"novelforge/backend/internal/infra/llm"
	"novelforge/backend/internal/infra/llm/prompts"
	metricservice "novelforge/backend/internal/service/metric"
)

// TxRunner 抽象对话确认流程所需的事务边界。
type TxRunner interface {
	InTx(ctx context.Context, fn func(context.Context) error) error
}

// Dependencies 声明对话(conversation)细化用例所需的依赖项。
type Dependencies struct {
	Conversations conversationdomain.ConversationRepository
	Projects      projectdomain.ProjectRepository
	Assets        assetdomain.AssetRepository
	LLMClient     llm.Client
	PromptStore   *prompts.Store
	Metrics       metricservice.UseCase
	TxRunner      TxRunner
}

// StartParams 定义发起细化对话所需参数。
type StartParams struct {
	ProjectID  string
	TargetType string
	TargetID   string
	Message    string
}

// ReplyParams 定义继续细化对话所需参数。
type ReplyParams struct {
	ConversationID string
	Message        string
}

// ListParams 定义项目下对话列表查询参数。
type ListParams struct {
	ProjectID  string
	TargetType string
	TargetID   string
	Limit      int
	Offset     int
}

// ConfirmResult 定义确认细化建议后的返回结果。
type ConfirmResult struct {
	Conversation *conversationdomain.Conversation
	Project      *projectdomain.Project
	Asset        *assetdomain.Asset
}

// UseCase 定义对话(conversation)细化应用边界。
type UseCase interface {
	Start(ctx context.Context, params StartParams) (*conversationdomain.Conversation, error)
	Reply(ctx context.Context, params ReplyParams) (*conversationdomain.Conversation, error)
	Confirm(ctx context.Context, conversationID string) (*ConfirmResult, error)
	GetByID(ctx context.Context, id string) (*conversationdomain.Conversation, error)
	List(ctx context.Context, params ListParams) ([]*conversationdomain.Conversation, error)
}
