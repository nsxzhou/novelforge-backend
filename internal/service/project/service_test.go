package project

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	projectdomain "inkmuse/backend/internal/domain/project"
	"inkmuse/backend/internal/infra/storage"
	memory "inkmuse/backend/internal/infra/storage/memory"
	appservice "inkmuse/backend/internal/service"
)

func testProjectInput() *projectdomain.Project {
	return &projectdomain.Project{
		Title:   "  Project Title  ",
		Summary: "  Project summary  ",
	}
}

func seedProject(t *testing.T, repo projectdomain.ProjectRepository) *projectdomain.Project {
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
		t.Fatalf("Create() error = %v", err)
	}
	return entity
}

type projectCreateConflictRepo struct{}

func (projectCreateConflictRepo) Create(context.Context, *projectdomain.Project) error {
	return storage.ErrAlreadyExists
}
func (projectCreateConflictRepo) GetByID(context.Context, string) (*projectdomain.Project, error) {
	return nil, nil
}
func (projectCreateConflictRepo) List(context.Context, projectdomain.ListParams) ([]*projectdomain.Project, error) {
	return nil, nil
}
func (projectCreateConflictRepo) Update(context.Context, *projectdomain.Project) error {
	return nil
}
func (projectCreateConflictRepo) UpdateIfUnchanged(context.Context, *projectdomain.Project, time.Time) (bool, error) {
	return false, nil
}

type projectListCaptureRepo struct {
	params projectdomain.ListParams
}

func (r *projectListCaptureRepo) Create(context.Context, *projectdomain.Project) error {
	return nil
}
func (r *projectListCaptureRepo) GetByID(context.Context, string) (*projectdomain.Project, error) {
	return nil, nil
}
func (r *projectListCaptureRepo) List(_ context.Context, params projectdomain.ListParams) ([]*projectdomain.Project, error) {
	r.params = params
	return []*projectdomain.Project{}, nil
}
func (r *projectListCaptureRepo) Update(context.Context, *projectdomain.Project) error {
	return nil
}
func (r *projectListCaptureRepo) UpdateIfUnchanged(context.Context, *projectdomain.Project, time.Time) (bool, error) {
	return false, nil
}

func TestCreateDefaultsAndTrimsProject(t *testing.T) {
	repo := memory.NewProjectRepository()
	uc := NewUseCase(Dependencies{Projects: repo})
	entity := testProjectInput()

	if err := uc.Create(context.Background(), entity); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if entity.ID == "" {
		t.Fatal("Create() ID = empty, want generated UUID")
	}
	if entity.Title != "Project Title" {
		t.Fatalf("Create() Title = %q, want trimmed value", entity.Title)
	}
	if entity.Summary != "Project summary" {
		t.Fatalf("Create() Summary = %q, want trimmed value", entity.Summary)
	}
	if entity.Status != projectdomain.StatusDraft {
		t.Fatalf("Create() Status = %q, want %q", entity.Status, projectdomain.StatusDraft)
	}
	if entity.CreatedAt.IsZero() || entity.UpdatedAt.IsZero() {
		t.Fatalf("Create() timestamps = %#v, want non-zero", entity)
	}
	if !entity.CreatedAt.Equal(entity.UpdatedAt) {
		t.Fatalf("Create() CreatedAt %v != UpdatedAt %v", entity.CreatedAt, entity.UpdatedAt)
	}
}

func TestCreateProjectConvertsConflict(t *testing.T) {
	uc := NewUseCase(Dependencies{Projects: projectCreateConflictRepo{}})

	err := uc.Create(context.Background(), testProjectInput())
	if !errors.Is(err, appservice.ErrConflict) {
		t.Fatalf("Create() error = %v, want ErrConflict", err)
	}
}

func TestGetByIDProjectConvertsNotFound(t *testing.T) {
	uc := NewUseCase(Dependencies{Projects: memory.NewProjectRepository()})

	_, err := uc.GetByID(context.Background(), uuid.NewString())
	if !errors.Is(err, appservice.ErrNotFound) {
		t.Fatalf("GetByID() error = %v, want ErrNotFound", err)
	}
}

func TestListProjectsValidatesStatusAndPassesPagination(t *testing.T) {
	repo := &projectListCaptureRepo{}
	uc := NewUseCase(Dependencies{Projects: repo})

	_, err := uc.List(context.Background(), projectdomain.ListParams{Status: "  ", Limit: 3, Offset: 7})
	if !errors.Is(err, appservice.ErrInvalidInput) {
		t.Fatalf("List() invalid status error = %v, want ErrInvalidInput", err)
	}

	_, err = uc.List(context.Background(), projectdomain.ListParams{Status: " active ", Limit: 3, Offset: 7})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if repo.params.Status != projectdomain.StatusActive {
		t.Fatalf("List() status = %q, want %q", repo.params.Status, projectdomain.StatusActive)
	}
	if repo.params.Limit != 3 || repo.params.Offset != 7 {
		t.Fatalf("List() params = %#v, want limit/offset passthrough", repo.params)
	}
}

func TestUpdateProjectPreservesImmutableFields(t *testing.T) {
	repo := memory.NewProjectRepository()
	seed := seedProject(t, repo)
	uc := NewUseCase(Dependencies{Projects: repo})

	update := &projectdomain.Project{
		ID:        seed.ID,
		Title:     "  Updated Project  ",
		Summary:   "  Updated summary  ",
		Status:    projectdomain.StatusActive,
		CreatedAt: time.Now().UTC().Add(time.Hour),
		UpdatedAt: seed.CreatedAt,
	}

	if err := uc.Update(context.Background(), update); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	stored, err := repo.GetByID(context.Background(), seed.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if stored.ID != seed.ID {
		t.Fatalf("Update() ID = %q, want %q", stored.ID, seed.ID)
	}
	if !stored.CreatedAt.Equal(seed.CreatedAt) {
		t.Fatalf("Update() CreatedAt = %v, want %v", stored.CreatedAt, seed.CreatedAt)
	}
	if stored.Title != "Updated Project" || stored.Summary != "Updated summary" {
		t.Fatalf("Update() stored = %#v, want trimmed fields", stored)
	}
	if stored.Status != projectdomain.StatusActive {
		t.Fatalf("Update() Status = %q, want %q", stored.Status, projectdomain.StatusActive)
	}
	if !stored.UpdatedAt.After(seed.UpdatedAt) {
		t.Fatalf("Update() UpdatedAt = %v, want after %v", stored.UpdatedAt, seed.UpdatedAt)
	}
}

func TestUpdateProjectConvertsNotFound(t *testing.T) {
	uc := NewUseCase(Dependencies{Projects: memory.NewProjectRepository()})

	err := uc.Update(context.Background(), &projectdomain.Project{
		ID:      uuid.NewString(),
		Title:   "Title",
		Summary: "Summary",
		Status:  projectdomain.StatusDraft,
	})
	if !errors.Is(err, appservice.ErrNotFound) {
		t.Fatalf("Update() error = %v, want ErrNotFound", err)
	}
}
