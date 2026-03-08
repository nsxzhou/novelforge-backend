package asset

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	assetdomain "novelforge/backend/internal/domain/asset"
	projectdomain "novelforge/backend/internal/domain/project"
	appservice "novelforge/backend/internal/service"
)

type useCase struct {
	assets   assetdomain.AssetRepository
	projects projectdomain.ProjectRepository
}

// NewUseCase 创建资产(asset)用例实现。
func NewUseCase(deps Dependencies) UseCase {
	return &useCase{
		assets:   deps.Assets,
		projects: deps.Projects,
	}
}

func (u *useCase) Create(ctx context.Context, entity *assetdomain.Asset) error {
	if entity == nil {
		return appservice.WrapInvalidInput(fmt.Errorf("asset must not be nil"))
	}

	entity.ProjectID = strings.TrimSpace(entity.ProjectID)
	entity.Type = strings.TrimSpace(entity.Type)
	if err := validateAssetProjectID(entity.ProjectID); err != nil {
		return err
	}
	if err := u.ensureProjectExists(ctx, entity.ProjectID); err != nil {
		return err
	}

	now := time.Now().UTC()
	entity.ID = uuid.NewString()
	entity.Title = strings.TrimSpace(entity.Title)
	entity.Content = strings.TrimSpace(entity.Content)
	entity.CreatedAt = now
	entity.UpdatedAt = now

	if err := entity.Validate(); err != nil {
		return appservice.WrapInvalidInput(err)
	}
	if err := u.assets.Create(ctx, entity); err != nil {
		return appservice.TranslateStorageError(err)
	}
	return nil
}

func (u *useCase) GetByID(ctx context.Context, id string) (*assetdomain.Asset, error) {
	if err := validateAssetID(id); err != nil {
		return nil, err
	}

	entity, err := u.assets.GetByID(ctx, strings.TrimSpace(id))
	if err != nil {
		return nil, appservice.TranslateStorageError(err)
	}
	return entity, nil
}

func (u *useCase) ListByProject(ctx context.Context, params assetdomain.ListByProjectParams) ([]*assetdomain.Asset, error) {
	params.ProjectID = strings.TrimSpace(params.ProjectID)
	if err := validateAssetProjectID(params.ProjectID); err != nil {
		return nil, err
	}
	if err := u.ensureProjectExists(ctx, params.ProjectID); err != nil {
		return nil, err
	}

	entities, err := u.assets.ListByProject(ctx, params)
	if err != nil {
		return nil, appservice.TranslateStorageError(err)
	}
	return entities, nil
}

func (u *useCase) ListByProjectAndType(ctx context.Context, params assetdomain.ListByProjectAndTypeParams) ([]*assetdomain.Asset, error) {
	params.ProjectID = strings.TrimSpace(params.ProjectID)
	if err := validateAssetProjectID(params.ProjectID); err != nil {
		return nil, err
	}
	if err := u.ensureProjectExists(ctx, params.ProjectID); err != nil {
		return nil, err
	}
	if params.Type != "" {
		trimmedType := strings.TrimSpace(params.Type)
		if trimmedType == "" {
			return nil, appservice.WrapInvalidInput(fmt.Errorf("type must not be empty"))
		}
		if !assetdomain.IsValidType(trimmedType) {
			return nil, appservice.WrapInvalidInput(fmt.Errorf("type must be one of worldbuilding, character, outline"))
		}
		params.Type = trimmedType
	}

	entities, err := u.assets.ListByProjectAndType(ctx, params)
	if err != nil {
		return nil, appservice.TranslateStorageError(err)
	}
	return entities, nil
}

func (u *useCase) Update(ctx context.Context, entity *assetdomain.Asset) error {
	if entity == nil {
		return appservice.WrapInvalidInput(fmt.Errorf("asset must not be nil"))
	}
	if err := validateAssetID(entity.ID); err != nil {
		return err
	}

	existing, err := u.assets.GetByID(ctx, strings.TrimSpace(entity.ID))
	if err != nil {
		return appservice.TranslateStorageError(err)
	}
	if err := u.ensureProjectExists(ctx, existing.ProjectID); err != nil {
		return err
	}

	entity.ID = existing.ID
	entity.ProjectID = existing.ProjectID
	entity.CreatedAt = existing.CreatedAt
	entity.Type = strings.TrimSpace(entity.Type)
	entity.Title = strings.TrimSpace(entity.Title)
	entity.Content = strings.TrimSpace(entity.Content)
	entity.UpdatedAt = time.Now().UTC()

	if err := entity.Validate(); err != nil {
		return appservice.WrapInvalidInput(err)
	}
	if err := u.assets.Update(ctx, entity); err != nil {
		return appservice.TranslateStorageError(err)
	}
	return nil
}

func (u *useCase) Delete(ctx context.Context, id string) error {
	if err := validateAssetID(id); err != nil {
		return err
	}
	if err := u.assets.Delete(ctx, strings.TrimSpace(id)); err != nil {
		return appservice.TranslateStorageError(err)
	}
	return nil
}

func (u *useCase) ensureProjectExists(ctx context.Context, projectID string) error {
	if _, err := u.projects.GetByID(ctx, projectID); err != nil {
		return appservice.TranslateStorageError(err)
	}
	return nil
}

func validateAssetID(id string) error {
	if _, err := uuid.Parse(strings.TrimSpace(id)); err != nil {
		return appservice.WrapInvalidInput(fmt.Errorf("id must be a valid UUID"))
	}
	return nil
}

func validateAssetProjectID(projectID string) error {
	if _, err := uuid.Parse(projectID); err != nil {
		return appservice.WrapInvalidInput(fmt.Errorf("project_id must be a valid UUID"))
	}
	return nil
}
