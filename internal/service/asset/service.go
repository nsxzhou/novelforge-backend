package asset

import (
	"context"

	assetdomain "inkmuse/backend/internal/domain/asset"
	generationdomain "inkmuse/backend/internal/domain/generation"
	projectdomain "inkmuse/backend/internal/domain/project"
	promptdomain "inkmuse/backend/internal/domain/prompt"
	"inkmuse/backend/internal/infra/llm"
	"inkmuse/backend/internal/infra/llm/prompts"
	metricservice "inkmuse/backend/internal/service/metric"

	"github.com/cloudwego/eino/schema"
)

// Dependencies 声明资产(asset)用例所需的领域依赖项。
type Dependencies struct {
	Assets            assetdomain.AssetRepository
	Projects          projectdomain.ProjectRepository
	GenerationRecords generationdomain.GenerationRecordRepository
	LLMClient         llm.Client
	PromptStore       *prompts.Store
	PromptOverrides   promptdomain.OverrideRepository
	Metrics           metricservice.UseCase
}

// GenerateParams 定义资产生成所需参数。
type GenerateParams struct {
	ProjectID   string
	Type        string
	Instruction string
}

// GenerateResult 定义资产生成结果。
type GenerateResult struct {
	Asset            *assetdomain.Asset
	GenerationRecord *generationdomain.GenerationRecord
}

// GenerateStreamResult 定义资产流式生成结果。
type GenerateStreamResult struct {
	Record     *generationdomain.GenerationRecord
	Stream     *schema.StreamReader[*schema.Message]
	OnComplete func(content string) (*GenerateResult, error)
	OnError    func(err error)
}

// UseCase 定义资产(asset)的应用边界。
type UseCase interface {
	Create(ctx context.Context, asset *assetdomain.Asset) error
	GetByID(ctx context.Context, id string) (*assetdomain.Asset, error)
	ListByProject(ctx context.Context, params assetdomain.ListByProjectParams) ([]*assetdomain.Asset, error)
	ListByProjectAndType(ctx context.Context, params assetdomain.ListByProjectAndTypeParams) ([]*assetdomain.Asset, error)
	Update(ctx context.Context, asset *assetdomain.Asset) error
	Delete(ctx context.Context, id string) error
	Generate(ctx context.Context, params GenerateParams) (*GenerateResult, error)
	GenerateStream(ctx context.Context, params GenerateParams) (*GenerateStreamResult, error)
}
