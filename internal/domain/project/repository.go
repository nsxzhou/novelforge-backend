package project

import (
	"context"
	"time"
)

// ListParams 定义项目列表过滤器。
type ListParams struct {
	Limit  int
	Offset int
	Status string
}

// ProjectRepository 定义项目(project)持久化行为。
type ProjectRepository interface {
	Create(ctx context.Context, project *Project) error
	GetByID(ctx context.Context, id string) (*Project, error)
	List(ctx context.Context, params ListParams) ([]*Project, error)
	Update(ctx context.Context, project *Project) error
	// UpdateIfUnchanged 使用 optimistic locking 避免并发请求覆盖最新项目状态。
	UpdateIfUnchanged(ctx context.Context, project *Project, expectedUpdatedAt time.Time) (bool, error)
}
