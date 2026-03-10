package asset

import (
	"context"

	assetdomain "novelforge/backend/internal/domain/asset"
	generationdomain "novelforge/backend/internal/domain/generation"
	projectdomain "novelforge/backend/internal/domain/project"
	"novelforge/backend/internal/infra/llm"
	"novelforge/backend/internal/infra/llm/prompts"
	metricservice "novelforge/backend/internal/service/metric"
)

// Dependencies 声明资产(asset)用例所需的领域依赖项。
type Dependencies struct {
	Assets            assetdomain.AssetRepository
	Projects          projectdomain.ProjectRepository
	GenerationRecords generationdomain.GenerationRecordRepository
	LLMClient         llm.Client
	PromptStore       *prompts.Store
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

// UseCase 定义资产(asset)的应用边界。
type UseCase interface {
	Create(ctx context.Context, asset *assetdomain.Asset) error
	GetByID(ctx context.Context, id string) (*assetdomain.Asset, error)
	ListByProject(ctx context.Context, params assetdomain.ListByProjectParams) ([]*assetdomain.Asset, error)
	ListByProjectAndType(ctx context.Context, params assetdomain.ListByProjectAndTypeParams) ([]*assetdomain.Asset, error)
	Update(ctx context.Context, asset *assetdomain.Asset) error
	Delete(ctx context.Context, id string) error
	Generate(ctx context.Context, params GenerateParams) (*GenerateResult, error)
}
