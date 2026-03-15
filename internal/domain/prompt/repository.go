package prompt

import "context"

// OverrideRepository 定义项目级 prompt 覆盖的持久化契约。
type OverrideRepository interface {
	ListByProject(ctx context.Context, projectID string) ([]*ProjectPromptOverride, error)
	GetByProjectAndCapability(ctx context.Context, projectID string, capability string) (*ProjectPromptOverride, error)
	Upsert(ctx context.Context, override *ProjectPromptOverride) error
	Delete(ctx context.Context, projectID string, capability string) error
}
