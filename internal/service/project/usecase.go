package project

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	projectdomain "novelforge/backend/internal/domain/project"
	appservice "novelforge/backend/internal/service"
)

type useCase struct {
	projects projectdomain.ProjectRepository
}

// NewUseCase 创建项目(project)用例实现。
func NewUseCase(deps Dependencies) UseCase {
	return &useCase{projects: deps.Projects}
}

func (u *useCase) Create(ctx context.Context, entity *projectdomain.Project) error {
	if entity == nil {
		return appservice.WrapInvalidInput(fmt.Errorf("project must not be nil"))
	}

	now := time.Now().UTC()
	entity.ID = uuid.NewString()
	entity.Title = strings.TrimSpace(entity.Title)
	entity.Summary = strings.TrimSpace(entity.Summary)
	entity.Status = strings.TrimSpace(entity.Status)
	if entity.Status == "" {
		entity.Status = projectdomain.StatusDraft
	}
	entity.CreatedAt = now
	entity.UpdatedAt = now

	if err := entity.Validate(); err != nil {
		return appservice.WrapInvalidInput(err)
	}
	if err := u.projects.Create(ctx, entity); err != nil {
		return appservice.TranslateStorageError(err)
	}
	return nil
}

func (u *useCase) GetByID(ctx context.Context, id string) (*projectdomain.Project, error) {
	if err := validateProjectID(id); err != nil {
		return nil, err
	}

	entity, err := u.projects.GetByID(ctx, strings.TrimSpace(id))
	if err != nil {
		return nil, appservice.TranslateStorageError(err)
	}
	return entity, nil
}

func (u *useCase) List(ctx context.Context, params projectdomain.ListParams) ([]*projectdomain.Project, error) {
	if params.Status != "" {
		trimmedStatus := strings.TrimSpace(params.Status)
		if trimmedStatus == "" {
			return nil, appservice.WrapInvalidInput(fmt.Errorf("status must not be empty"))
		}
		if !projectdomain.IsValidStatus(trimmedStatus) {
			return nil, appservice.WrapInvalidInput(fmt.Errorf("status must be one of draft, active, archived"))
		}
		params.Status = trimmedStatus
	}

	entities, err := u.projects.List(ctx, params)
	if err != nil {
		return nil, appservice.TranslateStorageError(err)
	}
	return entities, nil
}

func (u *useCase) Update(ctx context.Context, entity *projectdomain.Project) error {
	if entity == nil {
		return appservice.WrapInvalidInput(fmt.Errorf("project must not be nil"))
	}
	if err := validateProjectID(entity.ID); err != nil {
		return err
	}

	existing, err := u.projects.GetByID(ctx, strings.TrimSpace(entity.ID))
	if err != nil {
		return appservice.TranslateStorageError(err)
	}

	entity.ID = existing.ID
	entity.CreatedAt = existing.CreatedAt
	entity.Title = strings.TrimSpace(entity.Title)
	entity.Summary = strings.TrimSpace(entity.Summary)
	entity.Status = strings.TrimSpace(entity.Status)
	entity.UpdatedAt = time.Now().UTC()

	if err := entity.Validate(); err != nil {
		return appservice.WrapInvalidInput(err)
	}
	if err := u.projects.Update(ctx, entity); err != nil {
		return appservice.TranslateStorageError(err)
	}
	return nil
}

func validateProjectID(id string) error {
	if _, err := uuid.Parse(strings.TrimSpace(id)); err != nil {
		return appservice.WrapInvalidInput(fmt.Errorf("id must be a valid UUID"))
	}
	return nil
}
