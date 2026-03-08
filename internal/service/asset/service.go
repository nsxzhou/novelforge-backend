package asset

import (
	"context"

	assetdomain "novelforge/backend/internal/domain/asset"
	projectdomain "novelforge/backend/internal/domain/project"
)

// Dependencies 声明资产(asset)用例所需的领域依赖项。
type Dependencies struct {
	Assets   assetdomain.AssetRepository
	Projects projectdomain.ProjectRepository
}

// UseCase 定义资产(asset)的应用边界。
type UseCase interface {
	Create(ctx context.Context, asset *assetdomain.Asset) error
	GetByID(ctx context.Context, id string) (*assetdomain.Asset, error)
	ListByProject(ctx context.Context, params assetdomain.ListByProjectParams) ([]*assetdomain.Asset, error)
	ListByProjectAndType(ctx context.Context, params assetdomain.ListByProjectAndTypeParams) ([]*assetdomain.Asset, error)
	Update(ctx context.Context, asset *assetdomain.Asset) error
	Delete(ctx context.Context, id string) error
}
