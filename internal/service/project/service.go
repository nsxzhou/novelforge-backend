package project

import (
	"context"

	projectdomain "novelforge/backend/internal/domain/project"
)

// Dependencies 声明项目(project)用例所需的领域依赖项。
type Dependencies struct {
	Projects projectdomain.ProjectRepository
}

// UseCase 定义项目(project)的应用边界。
type UseCase interface {
	Create(ctx context.Context, project *projectdomain.Project) error
	GetByID(ctx context.Context, id string) (*projectdomain.Project, error)
	List(ctx context.Context, params projectdomain.ListParams) ([]*projectdomain.Project, error)
	Update(ctx context.Context, project *projectdomain.Project) error
}
