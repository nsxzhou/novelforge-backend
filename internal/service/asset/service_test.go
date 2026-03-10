package asset

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"

	assetdomain "novelforge/backend/internal/domain/asset"
	generationdomain "novelforge/backend/internal/domain/generation"
	metricdomain "novelforge/backend/internal/domain/metric"
	projectdomain "novelforge/backend/internal/domain/project"
	"novelforge/backend/internal/infra/llm/prompts"
	"novelforge/backend/internal/infra/storage"
	memory "novelforge/backend/internal/infra/storage/memory"
	appservice "novelforge/backend/internal/service"
	metricservice "novelforge/backend/internal/service/metric"
	"novelforge/backend/pkg/config"
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
func (assetCreateConflictRepo) UpdateIfUnchanged(context.Context, *assetdomain.Asset, time.Time) (bool, error) {
	return false, nil
}
func (assetCreateConflictRepo) Delete(context.Context, string) error {
	return nil
}

type stubChatModel struct {
	generate func(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error)
}

func (s *stubChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	if s.generate != nil {
		return s.generate(ctx, input, opts...)
	}
	return nil, errors.New("unexpected Generate call")
}

func (s *stubChatModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("unexpected Stream call")
}

func (s *stubChatModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return s, nil
}

type stubLLMClient struct {
	chatModel model.ToolCallingChatModel
}

func (s *stubLLMClient) Provider() string { return "stub" }
func (s *stubLLMClient) Model() string    { return "stub-model" }
func (s *stubLLMClient) ChatModel() model.ToolCallingChatModel {
	return s.chatModel
}

func loadTestPromptStore(t *testing.T) *prompts.Store {
	t.Helper()
	store, err := prompts.LoadStore(config.PromptConfig{
		"asset_generation":     "asset_generation.yaml",
		"chapter_generation":   "chapter_generation.yaml",
		"chapter_continuation": "chapter_continuation.yaml",
		"chapter_rewrite":      "chapter_rewrite.yaml",
		"project_refinement":   "project_refinement.yaml",
		"asset_refinement":     "asset_refinement.yaml",
	})
	if err != nil {
		t.Fatalf("LoadStore() error = %v", err)
	}
	return store
}

func newMetricUseCase(metricRepo metricdomain.MetricEventRepository) metricservice.UseCase {
	return metricservice.NewUseCase(metricservice.Dependencies{MetricEvents: metricRepo})
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

func TestGenerateAssetSuccessCreatesGenerationRecordAndMetric(t *testing.T) {
	projectRepo := memory.NewProjectRepository()
	project := seedProjectForAsset(t, projectRepo)
	assetRepo := memory.NewAssetRepository()
	generationRepo := memory.NewGenerationRecordRepository()
	metricRepo := memory.NewMetricEventRepository()
	uc := NewUseCase(Dependencies{
		Assets:            assetRepo,
		Projects:          projectRepo,
		GenerationRecords: generationRepo,
		LLMClient: &stubLLMClient{
			chatModel: &stubChatModel{
				generate: func(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
					return &schema.Message{Content: `{"title":"新主角设定","content":"沉着冷静，城府极深。"}`}, nil
				},
			},
		},
		PromptStore: loadTestPromptStore(t),
		Metrics:     newMetricUseCase(metricRepo),
	})

	result, err := uc.Generate(context.Background(), GenerateParams{
		ProjectID:   project.ID,
		Type:        assetdomain.TypeCharacter,
		Instruction: "生成一个冷静且有野心的主角设定。",
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if result == nil || result.Asset == nil || result.GenerationRecord == nil {
		t.Fatalf("Generate() result = %#v, want non-nil asset and generation record", result)
	}
	if result.Asset.ProjectID != project.ID || result.Asset.Type != assetdomain.TypeCharacter {
		t.Fatalf("generated asset = %#v, want project/type preserved", result.Asset)
	}
	if result.GenerationRecord.Kind != generationdomain.KindAssetGeneration || result.GenerationRecord.Status != generationdomain.StatusSucceeded {
		t.Fatalf("generated record = %#v, want succeeded asset_generation record", result.GenerationRecord)
	}
	if result.GenerationRecord.OutputRef != result.Asset.ID {
		t.Fatalf("generation output_ref = %q, want %q", result.GenerationRecord.OutputRef, result.Asset.ID)
	}

	storedAsset, err := assetRepo.GetByID(context.Background(), result.Asset.ID)
	if err != nil {
		t.Fatalf("GetByID(asset) error = %v", err)
	}
	if storedAsset.Title != "新主角设定" || storedAsset.Content != "沉着冷静，城府极深。" {
		t.Fatalf("stored asset = %#v, want generated title/content", storedAsset)
	}

	storedRecord, err := generationRepo.GetByID(context.Background(), result.GenerationRecord.ID)
	if err != nil {
		t.Fatalf("GetByID(generation) error = %v", err)
	}
	if storedRecord.Status != generationdomain.StatusSucceeded {
		t.Fatalf("stored generation status = %q, want %q", storedRecord.Status, generationdomain.StatusSucceeded)
	}

	events, err := metricRepo.ListByProject(context.Background(), metricdomain.ListByProjectParams{
		ProjectID: project.ID,
		EventName: metricdomain.EventOperationCompleted,
	})
	if err != nil {
		t.Fatalf("ListByProject(metric) error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("len(metric events) = %d, want 1", len(events))
	}
	if events[0].Labels["domain"] != "asset" || events[0].Labels["action"] != "generate" {
		t.Fatalf("metric labels = %#v, want asset/generate", events[0].Labels)
	}
	if events[0].Labels["generation_kind"] != generationdomain.KindAssetGeneration || events[0].Labels["asset_type"] != assetdomain.TypeCharacter {
		t.Fatalf("metric labels = %#v, want generation kind and asset type", events[0].Labels)
	}
}

func TestGenerateAssetInvalidJSONMarksGenerationFailed(t *testing.T) {
	projectRepo := memory.NewProjectRepository()
	project := seedProjectForAsset(t, projectRepo)
	generationRepo := memory.NewGenerationRecordRepository()
	metricRepo := memory.NewMetricEventRepository()
	uc := NewUseCase(Dependencies{
		Assets:            memory.NewAssetRepository(),
		Projects:          projectRepo,
		GenerationRecords: generationRepo,
		LLMClient: &stubLLMClient{
			chatModel: &stubChatModel{
				generate: func(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
					return &schema.Message{Content: "not json"}, nil
				},
			},
		},
		PromptStore: loadTestPromptStore(t),
		Metrics:     newMetricUseCase(metricRepo),
	})

	_, err := uc.Generate(context.Background(), GenerateParams{
		ProjectID:   project.ID,
		Type:        assetdomain.TypeOutline,
		Instruction: "生成一份剧情大纲。",
	})
	if !errors.Is(err, appservice.ErrInvalidInput) {
		t.Fatalf("Generate() error = %v, want ErrInvalidInput", err)
	}

	records, listErr := generationRepo.ListByProject(context.Background(), generationdomain.ListByProjectParams{
		ProjectID: project.ID,
	})
	if listErr != nil {
		t.Fatalf("ListByProject(generation) error = %v", listErr)
	}
	if len(records) != 1 {
		t.Fatalf("len(generation records) = %d, want 1", len(records))
	}
	if records[0].Status != generationdomain.StatusFailed {
		t.Fatalf("generation status = %q, want %q", records[0].Status, generationdomain.StatusFailed)
	}
	if !strings.Contains(records[0].ErrorMessage, "invalid llm json") {
		t.Fatalf("generation error_message = %q, want invalid llm json", records[0].ErrorMessage)
	}

	events, metricErr := metricRepo.ListByProject(context.Background(), metricdomain.ListByProjectParams{
		ProjectID: project.ID,
		EventName: metricdomain.EventOperationFailed,
	})
	if metricErr != nil {
		t.Fatalf("ListByProject(metric) error = %v", metricErr)
	}
	if len(events) != 1 {
		t.Fatalf("len(failed metric events) = %d, want 1", len(events))
	}
	if events[0].Labels["error_kind"] != "invalid_input" {
		t.Fatalf("metric labels = %#v, want invalid_input error_kind", events[0].Labels)
	}
}
