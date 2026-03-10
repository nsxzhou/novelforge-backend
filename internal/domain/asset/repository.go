package asset

import (
	"context"
	"time"
)

// ListByProjectParams 定义项目下的资产(asset)过滤器。
type ListByProjectParams struct {
	ProjectID string
	Limit     int
	Offset    int
}

// ListByProjectAndTypeParams 根据项目和类型定义资产(asset)过滤器。
type ListByProjectAndTypeParams struct {
	ProjectID string
	Type      string
	Limit     int
	Offset    int
}

// AssetRepository 定义资产(asset)持久化行为。
type AssetRepository interface {
	Create(ctx context.Context, asset *Asset) error
	GetByID(ctx context.Context, id string) (*Asset, error)
	ListByProject(ctx context.Context, params ListByProjectParams) ([]*Asset, error)
	ListByProjectAndType(ctx context.Context, params ListByProjectAndTypeParams) ([]*Asset, error)
	Update(ctx context.Context, asset *Asset) error
	// UpdateIfUnchanged 使用 optimistic locking 避免并发请求覆盖最新资产状态。
	UpdateIfUnchanged(ctx context.Context, asset *Asset, expectedUpdatedAt time.Time) (bool, error)
	Delete(ctx context.Context, id string) error
}
