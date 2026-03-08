package asset

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	assetdomain "novelforge/backend/internal/domain/asset"
	projectdomain "novelforge/backend/internal/domain/project"
	"novelforge/backend/internal/infra/storage"
	memory "novelforge/backend/internal/infra/storage/memory"
	appservice "novelforge/backend/internal/service"
)

func seedProjectForAsset(t *testing.T, repo projectdomain.ProjectRepository) *projectdomain.Project {
	t.Helper()

	now := time.Date(2026, 3, 6, 12, 0, 0, 0, time.UTC)
	entity := &projectdomain.Project{
		ID:        uuid.NewString(),
		Title:     "Seed Project",
		Summary:   "Seed summary",
		Status:    projectdomain.StatusDraft,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := repo.Create(context.Background(), entity); err != nil {
		t.Fatalf("Create() project error = %v", err)
	}
	return entity
}

func seedAsset(t *testing.T, repo assetdomain.AssetRepository, projectID string) *assetdomain.Asset {
	t.Helper()

	now := time.Date(2026, 3, 6, 12, 0, 0, 0, time.UTC)
	entity := &assetdomain.Asset{
		ID:        uuid.NewString(),
		ProjectID: projectID,
		Type:      assetdomain.TypeOutline,
		Title:     "Seed Asset",
		Content:   "Seed content",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := repo.Create(context.Background(), entity); err != nil {
		t.Fatalf("Create() asset error = %v", err)
	}
	return entity
}

type assetCreateConflictRepo struct{}

func (assetCreateConflictRepo) Create(context.Context, *assetdomain.Asset) error {
	return storage.ErrAlreadyExists
}
func (assetCreateConflictRepo) GetByID(context.Context, string) (*assetdomain.Asset, error) {
	return nil, nil
}
func (assetCreateConflictRepo) ListByProject(context.Context, assetdomain.ListByProjectParams) ([]*assetdomain.Asset, error) {
	return nil, nil
}
func (assetCreateConflictRepo) ListByProjectAndType(context.Context, assetdomain.ListByProjectAndTypeParams) ([]*assetdomain.Asset, error) {
	return nil, nil
}
func (assetCreateConflictRepo) Update(context.Context, *assetdomain.Asset) error {
	return nil
}
func (assetCreateConflictRepo) Delete(context.Context, string) error {
	return nil
}

func TestCreateAssetTrimsFieldsAndSetsTimestamps(t *testing.T) {
	projectRepo := memory.NewProjectRepository()
	project := seedProjectForAsset(t, projectRepo)
	assetRepo := memory.NewAssetRepository()
	uc := NewUseCase(Dependencies{Assets: assetRepo, Projects: projectRepo})
	entity := &assetdomain.Asset{
		ProjectID: "  " + project.ID + "  ",
		Type:      assetdomain.TypeCharacter,
		Title:     "  Hero  ",
		Content:   "  Main character  ",
	}

	if err := uc.Create(context.Background(), entity); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if entity.ID == "" {
		t.Fatal("Create() ID = empty, want generated UUID")
	}
	if entity.ProjectID != project.ID {
		t.Fatalf("Create() ProjectID = %q, want %q", entity.ProjectID, project.ID)
	}
	if entity.Title != "Hero" || entity.Content != "Main character" {
		t.Fatalf("Create() entity = %#v, want trimmed fields", entity)
	}
	if entity.CreatedAt.IsZero() || entity.UpdatedAt.IsZero() {
		t.Fatalf("Create() timestamps = %#v, want non-zero", entity)
	}
	if !entity.CreatedAt.Equal(entity.UpdatedAt) {
		t.Fatalf("Create() CreatedAt %v != UpdatedAt %v", entity.CreatedAt, entity.UpdatedAt)
	}
}

func TestCreateAssetRequiresParentProject(t *testing.T) {
	uc := NewUseCase(Dependencies{
		Assets:   memory.NewAssetRepository(),
		Projects: memory.NewProjectRepository(),
	})

	err := uc.Create(context.Background(), &assetdomain.Asset{
		ProjectID: uuid.NewString(),
		Type:      assetdomain.TypeOutline,
		Title:     "Outline",
		Content:   "Content",
	})
	if !errors.Is(err, appservice.ErrNotFound) {
		t.Fatalf("Create() error = %v, want ErrNotFound", err)
	}
}

func TestCreateAssetConvertsConflict(t *testing.T) {
	projectRepo := memory.NewProjectRepository()
	project := seedProjectForAsset(t, projectRepo)
	uc := NewUseCase(Dependencies{Assets: assetCreateConflictRepo{}, Projects: projectRepo})

	err := uc.Create(context.Background(), &assetdomain.Asset{
		ProjectID: project.ID,
		Type:      assetdomain.TypeOutline,
		Title:     "Outline",
		Content:   "Content",
	})
	if !errors.Is(err, appservice.ErrConflict) {
		t.Fatalf("Create() error = %v, want ErrConflict", err)
	}
}

func TestGetByIDAssetConvertsNotFound(t *testing.T) {
	uc := NewUseCase(Dependencies{
		Assets:   memory.NewAssetRepository(),
		Projects: memory.NewProjectRepository(),
	})

	_, err := uc.GetByID(context.Background(), uuid.NewString())
	if !errors.Is(err, appservice.ErrNotFound) {
		t.Fatalf("GetByID() error = %v, want ErrNotFound", err)
	}
}

func TestListAssetsByProjectAndTypeValidatesAndFilters(t *testing.T) {
	projectRepo := memory.NewProjectRepository()
	project := seedProjectForAsset(t, projectRepo)
	assetRepo := memory.NewAssetRepository()
	outline := seedAsset(t, assetRepo, project.ID)
	character := &assetdomain.Asset{
		ID:        uuid.NewString(),
		ProjectID: project.ID,
		Type:      assetdomain.TypeCharacter,
		Title:     "Hero",
		Content:   "Hero content",
		CreatedAt: time.Date(2026, 3, 6, 12, 1, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 3, 6, 12, 1, 0, 0, time.UTC),
	}
	if err := assetRepo.Create(context.Background(), character); err != nil {
		t.Fatalf("Create() asset error = %v", err)
	}
	uc := NewUseCase(Dependencies{Assets: assetRepo, Projects: projectRepo})

	_, err := uc.ListByProjectAndType(context.Background(), assetdomain.ListByProjectAndTypeParams{ProjectID: project.ID, Type: " invalid "})
	if !errors.Is(err, appservice.ErrInvalidInput) {
		t.Fatalf("ListByProjectAndType() invalid type error = %v, want ErrInvalidInput", err)
	}

	list, err := uc.ListByProjectAndType(context.Background(), assetdomain.ListByProjectAndTypeParams{
		ProjectID: project.ID,
		Type:      " character ",
		Limit:     5,
		Offset:    0,
	})
	if err != nil {
		t.Fatalf("ListByProjectAndType() error = %v", err)
	}
	if len(list) != 1 || list[0].ID != character.ID {
		t.Fatalf("ListByProjectAndType() = %#v, want only character asset", list)
	}
	if list[0].ID == outline.ID {
		t.Fatalf("ListByProjectAndType() returned outline asset = %#v", list)
	}
}

func TestListAssetsByProjectRequiresParentProject(t *testing.T) {
	uc := NewUseCase(Dependencies{
		Assets:   memory.NewAssetRepository(),
		Projects: memory.NewProjectRepository(),
	})

	_, err := uc.ListByProject(context.Background(), assetdomain.ListByProjectParams{ProjectID: uuid.NewString()})
	if !errors.Is(err, appservice.ErrNotFound) {
		t.Fatalf("ListByProject() error = %v, want ErrNotFound", err)
	}
}

func TestUpdateAssetPreservesImmutableFields(t *testing.T) {
	projectRepo := memory.NewProjectRepository()
	project := seedProjectForAsset(t, projectRepo)
	assetRepo := memory.NewAssetRepository()
	seed := seedAsset(t, assetRepo, project.ID)
	uc := NewUseCase(Dependencies{Assets: assetRepo, Projects: projectRepo})

	update := &assetdomain.Asset{
		ID:        seed.ID,
		ProjectID: uuid.NewString(),
		Type:      assetdomain.TypeWorldbuilding,
		Title:     "  Updated Asset  ",
		Content:   "  Updated content  ",
		CreatedAt: time.Now().UTC().Add(time.Hour),
		UpdatedAt: seed.CreatedAt,
	}

	if err := uc.Update(context.Background(), update); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	stored, err := assetRepo.GetByID(context.Background(), seed.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if stored.ID != seed.ID {
		t.Fatalf("Update() ID = %q, want %q", stored.ID, seed.ID)
	}
	if stored.ProjectID != seed.ProjectID {
		t.Fatalf("Update() ProjectID = %q, want %q", stored.ProjectID, seed.ProjectID)
	}
	if !stored.CreatedAt.Equal(seed.CreatedAt) {
		t.Fatalf("Update() CreatedAt = %v, want %v", stored.CreatedAt, seed.CreatedAt)
	}
	if stored.Type != assetdomain.TypeWorldbuilding {
		t.Fatalf("Update() Type = %q, want %q", stored.Type, assetdomain.TypeWorldbuilding)
	}
	if stored.Title != "Updated Asset" || stored.Content != "Updated content" {
		t.Fatalf("Update() stored = %#v, want trimmed fields", stored)
	}
	if !stored.UpdatedAt.After(seed.UpdatedAt) {
		t.Fatalf("Update() UpdatedAt = %v, want after %v", stored.UpdatedAt, seed.UpdatedAt)
	}
}

func TestUpdateAssetRequiresParentProject(t *testing.T) {
	projectRepo := memory.NewProjectRepository()
	project := seedProjectForAsset(t, projectRepo)
	assetRepo := memory.NewAssetRepository()
	seed := seedAsset(t, assetRepo, project.ID)
	missingProjectRepo := memory.NewProjectRepository()
	uc := NewUseCase(Dependencies{Assets: assetRepo, Projects: missingProjectRepo})

	err := uc.Update(context.Background(), &assetdomain.Asset{
		ID:      seed.ID,
		Type:    assetdomain.TypeOutline,
		Title:   "Updated",
		Content: "Updated",
	})
	if !errors.Is(err, appservice.ErrNotFound) {
		t.Fatalf("Update() error = %v, want ErrNotFound", err)
	}
}

func TestDeleteAssetConvertsNotFound(t *testing.T) {
	uc := NewUseCase(Dependencies{
		Assets:   memory.NewAssetRepository(),
		Projects: memory.NewProjectRepository(),
	})

	err := uc.Delete(context.Background(), uuid.NewString())
	if !errors.Is(err, appservice.ErrNotFound) {
		t.Fatalf("Delete() error = %v, want ErrNotFound", err)
	}
}
