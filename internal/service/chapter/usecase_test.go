package chapter

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	assetdomain "novelforge/backend/internal/domain/asset"
	chapterdomain "novelforge/backend/internal/domain/chapter"
	generationdomain "novelforge/backend/internal/domain/generation"
	projectdomain "novelforge/backend/internal/domain/project"
	"novelforge/backend/internal/infra/llm/prompts"
	"novelforge/backend/internal/infra/storage/memory"
	appservice "novelforge/backend/internal/service"
	"novelforge/backend/pkg/config"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

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

func createProjectEntity(t *testing.T, repo projectdomain.ProjectRepository, id string) *projectdomain.Project {
	t.Helper()
	now := time.Date(2026, 3, 9, 10, 0, 0, 0, time.UTC)
	entity := &projectdomain.Project{
		ID:        id,
		Title:     "NovelForge",
		Summary:   "A long-form fantasy adventure.",
		Status:    projectdomain.StatusDraft,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := repo.Create(context.Background(), entity); err != nil {
		t.Fatalf("Create(project) error = %v", err)
	}
	return entity
}

func createAssetEntity(t *testing.T, repo assetdomain.AssetRepository, id, projectID, assetType, title, content string) *assetdomain.Asset {
	t.Helper()
	now := time.Date(2026, 3, 9, 11, 0, 0, 0, time.UTC)
	entity := &assetdomain.Asset{
		ID:        id,
		ProjectID: projectID,
		Type:      assetType,
		Title:     title,
		Content:   content,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := repo.Create(context.Background(), entity); err != nil {
		t.Fatalf("Create(asset) error = %v", err)
	}
	return entity
}

func createChapterEntity(t *testing.T, repo chapterdomain.ChapterRepository, entity *chapterdomain.Chapter) *chapterdomain.Chapter {
	t.Helper()
	if err := repo.Create(context.Background(), entity); err != nil {
		t.Fatalf("Create(chapter) error = %v", err)
	}
	return entity
}

func createGenerationRecordEntity(t *testing.T, repo generationdomain.GenerationRecordRepository, entity *generationdomain.GenerationRecord) *generationdomain.GenerationRecord {
	t.Helper()
	if err := repo.Create(context.Background(), entity); err != nil {
		t.Fatalf("Create(generation_record) error = %v", err)
	}
	return entity
}

func TestUseCaseGenerateCreatesChapterAndGenerationRecord(t *testing.T) {
	projectRepo := memory.NewProjectRepository()
	assetRepo := memory.NewAssetRepository()
	chapterRepo := memory.NewChapterRepository()
	generationRepo := memory.NewGenerationRecordRepository()
	project := createProjectEntity(t, projectRepo, "11111111-1111-1111-1111-111111111111")
	createAssetEntity(t, assetRepo, "22222222-2222-2222-2222-222222222222", project.ID, assetdomain.TypeOutline, "主线大纲", "第一章主角踏入王城。")
	createAssetEntity(t, assetRepo, "33333333-3333-3333-3333-333333333333", project.ID, assetdomain.TypeWorldbuilding, "世界观", "王城位于浮空大陆。")
	createAssetEntity(t, assetRepo, "44444444-4444-4444-4444-444444444444", project.ID, assetdomain.TypeCharacter, "主角", "林澈，谨慎但好奇。")
	promptStore := loadTestPromptStore(t)

	var gotMessages []*schema.Message
	useCase := NewUseCase(Dependencies{
		Chapters:          chapterRepo,
		Projects:          projectRepo,
		Assets:            assetRepo,
		GenerationRecords: generationRepo,
		PromptStore:       promptStore,
		LLMClient: &stubLLMClient{chatModel: &stubChatModel{generate: func(_ context.Context, input []*schema.Message, _ ...model.Option) (*schema.Message, error) {
			gotMessages = input
			return &schema.Message{Content: "这是完整的第一章正文。"}, nil
		}}},
	})

	result, err := useCase.Generate(context.Background(), GenerateParams{
		ProjectID:   project.ID,
		Title:       "第一章 王城初见",
		Ordinal:     1,
		Instruction: "写出主角第一次进入王城时的压迫感与好奇心。",
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if result.Chapter.ProjectID != project.ID || result.Chapter.Title != "第一章 王城初见" || result.Chapter.Ordinal != 1 {
		t.Fatalf("chapter = %#v, want generated chapter metadata", result.Chapter)
	}
	if result.Chapter.Status != chapterdomain.StatusDraft || result.Chapter.Content != "这是完整的第一章正文。" {
		t.Fatalf("chapter = %#v, want draft chapter with generated content", result.Chapter)
	}
	if result.Chapter.CurrentDraftID != result.GenerationRecord.ID {
		t.Fatalf("CurrentDraftID = %q, want %q", result.Chapter.CurrentDraftID, result.GenerationRecord.ID)
	}
	if result.Chapter.CurrentDraftConfirmedAt != nil || result.Chapter.CurrentDraftConfirmedBy != "" {
		t.Fatalf("chapter confirmation fields = %#v/%q, want cleared", result.Chapter.CurrentDraftConfirmedAt, result.Chapter.CurrentDraftConfirmedBy)
	}
	if result.GenerationRecord.Kind != generationdomain.KindChapterGeneration || result.GenerationRecord.Status != generationdomain.StatusSucceeded {
		t.Fatalf("generation record = %#v, want succeeded chapter_generation record", result.GenerationRecord)
	}
	if result.GenerationRecord.TokenUsage != 0 || result.GenerationRecord.OutputRef != "这是完整的第一章正文。" {
		t.Fatalf("generation record = %#v, want token_usage=0 and output_ref set", result.GenerationRecord)
	}
	if result.GenerationRecord.DurationMillis < 0 {
		t.Fatalf("DurationMillis = %d, want >= 0", result.GenerationRecord.DurationMillis)
	}

	storedChapter, err := chapterRepo.GetByID(context.Background(), result.Chapter.ID)
	if err != nil {
		t.Fatalf("GetByID(chapter) error = %v", err)
	}
	if storedChapter.CurrentDraftID != result.GenerationRecord.ID || storedChapter.Content != "这是完整的第一章正文。" {
		t.Fatalf("stored chapter = %#v, want persisted generated chapter", storedChapter)
	}

	storedRecord, err := generationRepo.GetByID(context.Background(), result.GenerationRecord.ID)
	if err != nil {
		t.Fatalf("GetByID(generation) error = %v", err)
	}
	if storedRecord.Status != generationdomain.StatusSucceeded || storedRecord.ChapterID != result.Chapter.ID {
		t.Fatalf("stored record = %#v, want succeeded persisted record", storedRecord)
	}

	if len(gotMessages) != 2 || gotMessages[0].Role != schema.System || gotMessages[1].Role != schema.User {
		t.Fatalf("LLM messages = %#v, want system and user prompt", gotMessages)
	}
	userPrompt := gotMessages[1].Content
	for _, fragment := range []string{"第一章主角踏入王城", "浮空大陆", "林澈", "第一章 王城初见", "写出主角第一次进入王城"} {
		if !strings.Contains(userPrompt, fragment) {
			t.Fatalf("user prompt %q missing fragment %q", userPrompt, fragment)
		}
	}
}

func TestUseCaseGenerateRejectsDuplicateOrdinal(t *testing.T) {
	projectRepo := memory.NewProjectRepository()
	assetRepo := memory.NewAssetRepository()
	chapterRepo := memory.NewChapterRepository()
	generationRepo := memory.NewGenerationRecordRepository()
	project := createProjectEntity(t, projectRepo, "11111111-1111-1111-1111-111111111111")
	createChapterEntity(t, chapterRepo, &chapterdomain.Chapter{
		ID:        "22222222-2222-2222-2222-222222222222",
		ProjectID: project.ID,
		Title:     "已存在章节",
		Ordinal:   1,
		Status:    chapterdomain.StatusDraft,
		Content:   "已有内容",
		CreatedAt: time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC),
	})

	useCase := NewUseCase(Dependencies{
		Chapters:          chapterRepo,
		Projects:          projectRepo,
		Assets:            assetRepo,
		GenerationRecords: generationRepo,
		PromptStore:       loadTestPromptStore(t),
		LLMClient:         &stubLLMClient{chatModel: &stubChatModel{}},
	})

	_, err := useCase.Generate(context.Background(), GenerateParams{
		ProjectID:   project.ID,
		Title:       "重复序号章节",
		Ordinal:     1,
		Instruction: "继续写。",
	})
	if !errors.Is(err, appservice.ErrInvalidInput) {
		t.Fatalf("Generate() error = %v, want invalid input", err)
	}
	if !strings.Contains(err.Error(), "ordinal already exists") {
		t.Fatalf("Generate() error = %v, want duplicate ordinal message", err)
	}
	records, err := generationRepo.ListByProject(context.Background(), generationdomain.ListByProjectParams{ProjectID: project.ID})
	if err != nil {
		t.Fatalf("ListByProject(generation) error = %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("len(records) = %d, want 0", len(records))
	}
}

func TestUseCaseContinueResetsConfirmationAndPersistsRecord(t *testing.T) {
	projectRepo := memory.NewProjectRepository()
	assetRepo := memory.NewAssetRepository()
	chapterRepo := memory.NewChapterRepository()
	generationRepo := memory.NewGenerationRecordRepository()
	project := createProjectEntity(t, projectRepo, "11111111-1111-1111-1111-111111111111")
	createAssetEntity(t, assetRepo, "22222222-2222-2222-2222-222222222222", project.ID, assetdomain.TypeOutline, "主线大纲", "主角准备离开王城。")
	confirmedAt := time.Now().UTC().Add(-time.Minute)
	chapter := createChapterEntity(t, chapterRepo, &chapterdomain.Chapter{
		ID:                      "33333333-3333-3333-3333-333333333333",
		ProjectID:               project.ID,
		Title:                   "第一章",
		Ordinal:                 1,
		Status:                  chapterdomain.StatusConfirmed,
		Content:                 "旧的章节正文。",
		CurrentDraftID:          "44444444-4444-4444-4444-444444444444",
		CurrentDraftConfirmedAt: &confirmedAt,
		CurrentDraftConfirmedBy: "55555555-5555-5555-5555-555555555555",
		CreatedAt:               confirmedAt.Add(-time.Hour),
		UpdatedAt:               confirmedAt,
	})

	useCase := NewUseCase(Dependencies{
		Chapters:          chapterRepo,
		Projects:          projectRepo,
		Assets:            assetRepo,
		GenerationRecords: generationRepo,
		PromptStore:       loadTestPromptStore(t),
		LLMClient: &stubLLMClient{chatModel: &stubChatModel{generate: func(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
			return &schema.Message{Content: "新的完整章节正文。"}, nil
		}}},
	})

	result, err := useCase.Continue(context.Background(), ContinueParams{ChapterID: chapter.ID, Instruction: "继续推进主角离城前的冲突。"})
	if err != nil {
		t.Fatalf("Continue() error = %v", err)
	}
	if result.Chapter.Status != chapterdomain.StatusDraft || result.Chapter.Content != "新的完整章节正文。" {
		t.Fatalf("chapter = %#v, want updated draft chapter", result.Chapter)
	}
	if result.Chapter.CurrentDraftID == "44444444-4444-4444-4444-444444444444" || result.Chapter.CurrentDraftID != result.GenerationRecord.ID {
		t.Fatalf("CurrentDraftID = %q, want new generation record id", result.Chapter.CurrentDraftID)
	}
	if result.Chapter.CurrentDraftConfirmedAt != nil || result.Chapter.CurrentDraftConfirmedBy != "" {
		t.Fatalf("chapter confirmation fields = %#v/%q, want cleared", result.Chapter.CurrentDraftConfirmedAt, result.Chapter.CurrentDraftConfirmedBy)
	}
	stored, err := chapterRepo.GetByID(context.Background(), chapter.ID)
	if err != nil {
		t.Fatalf("GetByID(chapter) error = %v", err)
	}
	if stored.Content != "新的完整章节正文。" || stored.Status != chapterdomain.StatusDraft {
		t.Fatalf("stored chapter = %#v, want updated draft content", stored)
	}
	if result.GenerationRecord.Kind != generationdomain.KindChapterContinuation || result.GenerationRecord.Status != generationdomain.StatusSucceeded {
		t.Fatalf("generation record = %#v, want succeeded continuation record", result.GenerationRecord)
	}
}

func TestUseCaseConfirmMarksChapterAsConfirmed(t *testing.T) {
	projectRepo := memory.NewProjectRepository()
	assetRepo := memory.NewAssetRepository()
	chapterRepo := memory.NewChapterRepository()
	generationRepo := memory.NewGenerationRecordRepository()
	project := createProjectEntity(t, projectRepo, "11111111-1111-1111-1111-111111111111")
	chapter := createChapterEntity(t, chapterRepo, &chapterdomain.Chapter{
		ID:             "22222222-2222-2222-2222-222222222222",
		ProjectID:      project.ID,
		Title:          "第一章",
		Ordinal:        1,
		Status:         chapterdomain.StatusDraft,
		Content:        "当前章节正文。",
		CurrentDraftID: "33333333-3333-3333-3333-333333333333",
		CreatedAt:      time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC),
		UpdatedAt:      time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC),
	})
	createGenerationRecordEntity(t, generationRepo, &generationdomain.GenerationRecord{
		ID:               chapter.CurrentDraftID,
		ProjectID:        project.ID,
		ChapterID:        chapter.ID,
		Kind:             generationdomain.KindChapterGeneration,
		Status:           generationdomain.StatusSucceeded,
		InputSnapshotRef: "prompt snapshot",
		OutputRef:        "当前章节正文。",
		TokenUsage:       0,
		DurationMillis:   0,
		ErrorMessage:     "",
		CreatedAt:        chapter.CreatedAt,
		UpdatedAt:        chapter.UpdatedAt,
	})
	useCase := NewUseCase(Dependencies{
		Chapters:          chapterRepo,
		Projects:          projectRepo,
		Assets:            assetRepo,
		GenerationRecords: generationRepo,
	})

	confirmedBy := "44444444-4444-4444-4444-444444444444"
	result, err := useCase.Confirm(context.Background(), ConfirmParams{ChapterID: chapter.ID, ConfirmedBy: confirmedBy})
	if err != nil {
		t.Fatalf("Confirm() error = %v", err)
	}
	if result.Status != chapterdomain.StatusConfirmed {
		t.Fatalf("Status = %q, want %q", result.Status, chapterdomain.StatusConfirmed)
	}
	if result.CurrentDraftID != chapter.CurrentDraftID {
		t.Fatalf("CurrentDraftID = %q, want %q", result.CurrentDraftID, chapter.CurrentDraftID)
	}
	if result.CurrentDraftConfirmedAt == nil {
		t.Fatal("CurrentDraftConfirmedAt = nil, want timestamp")
	}
	if result.CurrentDraftConfirmedBy != confirmedBy {
		t.Fatalf("CurrentDraftConfirmedBy = %q, want %q", result.CurrentDraftConfirmedBy, confirmedBy)
	}

	stored, err := chapterRepo.GetByID(context.Background(), chapter.ID)
	if err != nil {
		t.Fatalf("GetByID(chapter) error = %v", err)
	}
	if stored.Status != chapterdomain.StatusConfirmed {
		t.Fatalf("stored status = %q, want %q", stored.Status, chapterdomain.StatusConfirmed)
	}
	if stored.CurrentDraftConfirmedAt == nil {
		t.Fatal("stored CurrentDraftConfirmedAt = nil, want timestamp")
	}
	if stored.CurrentDraftConfirmedBy != confirmedBy {
		t.Fatalf("stored CurrentDraftConfirmedBy = %q, want %q", stored.CurrentDraftConfirmedBy, confirmedBy)
	}
}

func TestUseCaseConfirmReturnsCurrentChapterWhenDraftAlreadyConfirmed(t *testing.T) {
	projectRepo := memory.NewProjectRepository()
	assetRepo := memory.NewAssetRepository()
	chapterRepo := memory.NewChapterRepository()
	generationRepo := memory.NewGenerationRecordRepository()
	project := createProjectEntity(t, projectRepo, "11111111-1111-1111-1111-111111111111")
	confirmedAt := time.Date(2026, 3, 9, 12, 5, 0, 0, time.UTC)
	chapter := createChapterEntity(t, chapterRepo, &chapterdomain.Chapter{
		ID:                      "22222222-2222-2222-2222-222222222222",
		ProjectID:               project.ID,
		Title:                   "第一章",
		Ordinal:                 1,
		Status:                  chapterdomain.StatusConfirmed,
		Content:                 "当前章节正文。",
		CurrentDraftID:          "33333333-3333-3333-3333-333333333333",
		CurrentDraftConfirmedAt: &confirmedAt,
		CurrentDraftConfirmedBy: "44444444-4444-4444-4444-444444444444",
		CreatedAt:               time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC),
		UpdatedAt:               confirmedAt,
	})
	createGenerationRecordEntity(t, generationRepo, &generationdomain.GenerationRecord{
		ID:               chapter.CurrentDraftID,
		ProjectID:        project.ID,
		ChapterID:        chapter.ID,
		Kind:             generationdomain.KindChapterGeneration,
		Status:           generationdomain.StatusSucceeded,
		InputSnapshotRef: "prompt snapshot",
		OutputRef:        chapter.Content,
		TokenUsage:       0,
		DurationMillis:   0,
		ErrorMessage:     "",
		CreatedAt:        chapter.CreatedAt,
		UpdatedAt:        chapter.UpdatedAt,
	})
	useCase := NewUseCase(Dependencies{
		Chapters:          chapterRepo,
		Projects:          projectRepo,
		Assets:            assetRepo,
		GenerationRecords: generationRepo,
	})

	result, err := useCase.Confirm(context.Background(), ConfirmParams{ChapterID: chapter.ID, ConfirmedBy: "55555555-5555-5555-5555-555555555555"})
	if err != nil {
		t.Fatalf("Confirm() error = %v", err)
	}
	if result.CurrentDraftConfirmedBy != chapter.CurrentDraftConfirmedBy {
		t.Fatalf("CurrentDraftConfirmedBy = %q, want %q", result.CurrentDraftConfirmedBy, chapter.CurrentDraftConfirmedBy)
	}
	if result.CurrentDraftConfirmedAt == nil || !result.CurrentDraftConfirmedAt.Equal(confirmedAt) {
		t.Fatalf("CurrentDraftConfirmedAt = %#v, want %v", result.CurrentDraftConfirmedAt, confirmedAt)
	}
}

func TestUseCaseConfirmRejectsMissingCurrentDraft(t *testing.T) {
	projectRepo := memory.NewProjectRepository()
	assetRepo := memory.NewAssetRepository()
	chapterRepo := memory.NewChapterRepository()
	generationRepo := memory.NewGenerationRecordRepository()
	project := createProjectEntity(t, projectRepo, "11111111-1111-1111-1111-111111111111")
	chapter := createChapterEntity(t, chapterRepo, &chapterdomain.Chapter{
		ID:        "22222222-2222-2222-2222-222222222222",
		ProjectID: project.ID,
		Title:     "第一章",
		Ordinal:   1,
		Status:    chapterdomain.StatusDraft,
		Content:   "当前章节正文。",
		CreatedAt: time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC),
	})
	useCase := NewUseCase(Dependencies{
		Chapters:          chapterRepo,
		Projects:          projectRepo,
		Assets:            assetRepo,
		GenerationRecords: generationRepo,
	})

	_, err := useCase.Confirm(context.Background(), ConfirmParams{ChapterID: chapter.ID, ConfirmedBy: "33333333-3333-3333-3333-333333333333"})
	if !errors.Is(err, appservice.ErrConflict) {
		t.Fatalf("Confirm() error = %v, want conflict", err)
	}
	if !strings.Contains(err.Error(), "current_draft_id must not be empty") {
		t.Fatalf("Confirm() error = %v, want current draft conflict", err)
	}
}

func TestUseCaseConfirmRejectsInvalidConfirmedBy(t *testing.T) {
	useCase := NewUseCase(Dependencies{
		Chapters:          memory.NewChapterRepository(),
		Projects:          memory.NewProjectRepository(),
		Assets:            memory.NewAssetRepository(),
		GenerationRecords: memory.NewGenerationRecordRepository(),
	})

	_, err := useCase.Confirm(context.Background(), ConfirmParams{ChapterID: "11111111-1111-1111-1111-111111111111", ConfirmedBy: "not-a-uuid"})
	if !errors.Is(err, appservice.ErrInvalidInput) {
		t.Fatalf("Confirm() error = %v, want invalid input", err)
	}
	if !strings.Contains(err.Error(), "confirmed_by must be a valid UUID") {
		t.Fatalf("Confirm() error = %v, want confirmed_by validation", err)
	}
}

func TestUseCaseConfirmReturnsNotFoundWhenGenerationRecordMissing(t *testing.T) {
	projectRepo := memory.NewProjectRepository()
	assetRepo := memory.NewAssetRepository()
	chapterRepo := memory.NewChapterRepository()
	generationRepo := memory.NewGenerationRecordRepository()
	project := createProjectEntity(t, projectRepo, "11111111-1111-1111-1111-111111111111")
	chapter := createChapterEntity(t, chapterRepo, &chapterdomain.Chapter{
		ID:             "22222222-2222-2222-2222-222222222222",
		ProjectID:      project.ID,
		Title:          "第一章",
		Ordinal:        1,
		Status:         chapterdomain.StatusDraft,
		Content:        "当前章节正文。",
		CurrentDraftID: "33333333-3333-3333-3333-333333333333",
		CreatedAt:      time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC),
		UpdatedAt:      time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC),
	})
	useCase := NewUseCase(Dependencies{
		Chapters:          chapterRepo,
		Projects:          projectRepo,
		Assets:            assetRepo,
		GenerationRecords: generationRepo,
	})

	_, err := useCase.Confirm(context.Background(), ConfirmParams{ChapterID: chapter.ID, ConfirmedBy: "44444444-4444-4444-4444-444444444444"})
	if !errors.Is(err, appservice.ErrNotFound) {
		t.Fatalf("Confirm() error = %v, want not found", err)
	}
}

func TestUseCaseConfirmRejectsUnsucceededGenerationRecord(t *testing.T) {
	projectRepo := memory.NewProjectRepository()
	assetRepo := memory.NewAssetRepository()
	chapterRepo := memory.NewChapterRepository()
	generationRepo := memory.NewGenerationRecordRepository()
	project := createProjectEntity(t, projectRepo, "11111111-1111-1111-1111-111111111111")
	chapter := createChapterEntity(t, chapterRepo, &chapterdomain.Chapter{
		ID:             "22222222-2222-2222-2222-222222222222",
		ProjectID:      project.ID,
		Title:          "第一章",
		Ordinal:        1,
		Status:         chapterdomain.StatusDraft,
		Content:        "当前章节正文。",
		CurrentDraftID: "33333333-3333-3333-3333-333333333333",
		CreatedAt:      time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC),
		UpdatedAt:      time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC),
	})
	createGenerationRecordEntity(t, generationRepo, &generationdomain.GenerationRecord{
		ID:               chapter.CurrentDraftID,
		ProjectID:        project.ID,
		ChapterID:        chapter.ID,
		Kind:             generationdomain.KindChapterGeneration,
		Status:           generationdomain.StatusRunning,
		InputSnapshotRef: "prompt snapshot",
		OutputRef:        "",
		TokenUsage:       0,
		DurationMillis:   0,
		ErrorMessage:     "",
		CreatedAt:        chapter.CreatedAt,
		UpdatedAt:        chapter.UpdatedAt,
	})
	useCase := NewUseCase(Dependencies{
		Chapters:          chapterRepo,
		Projects:          projectRepo,
		Assets:            assetRepo,
		GenerationRecords: generationRepo,
	})

	_, err := useCase.Confirm(context.Background(), ConfirmParams{ChapterID: chapter.ID, ConfirmedBy: "44444444-4444-4444-4444-444444444444"})
	if !errors.Is(err, appservice.ErrConflict) {
		t.Fatalf("Confirm() error = %v, want conflict", err)
	}
	if !strings.Contains(err.Error(), "must be succeeded") {
		t.Fatalf("Confirm() error = %v, want succeeded status conflict", err)
	}
}

func TestUseCaseConfirmRejectsGenerationRecordChapterMismatch(t *testing.T) {
	projectRepo := memory.NewProjectRepository()
	assetRepo := memory.NewAssetRepository()
	chapterRepo := memory.NewChapterRepository()
	generationRepo := memory.NewGenerationRecordRepository()
	project := createProjectEntity(t, projectRepo, "11111111-1111-1111-1111-111111111111")
	chapter := createChapterEntity(t, chapterRepo, &chapterdomain.Chapter{
		ID:             "22222222-2222-2222-2222-222222222222",
		ProjectID:      project.ID,
		Title:          "第一章",
		Ordinal:        1,
		Status:         chapterdomain.StatusDraft,
		Content:        "当前章节正文。",
		CurrentDraftID: "33333333-3333-3333-3333-333333333333",
		CreatedAt:      time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC),
		UpdatedAt:      time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC),
	})
	createGenerationRecordEntity(t, generationRepo, &generationdomain.GenerationRecord{
		ID:               chapter.CurrentDraftID,
		ProjectID:        project.ID,
		ChapterID:        "55555555-5555-5555-5555-555555555555",
		Kind:             generationdomain.KindChapterContinuation,
		Status:           generationdomain.StatusSucceeded,
		InputSnapshotRef: "prompt snapshot",
		OutputRef:        "当前章节正文。",
		TokenUsage:       0,
		DurationMillis:   0,
		ErrorMessage:     "",
		CreatedAt:        chapter.CreatedAt,
		UpdatedAt:        chapter.UpdatedAt,
	})
	useCase := NewUseCase(Dependencies{
		Chapters:          chapterRepo,
		Projects:          projectRepo,
		Assets:            assetRepo,
		GenerationRecords: generationRepo,
	})

	_, err := useCase.Confirm(context.Background(), ConfirmParams{ChapterID: chapter.ID, ConfirmedBy: "44444444-4444-4444-4444-444444444444"})
	if !errors.Is(err, appservice.ErrConflict) {
		t.Fatalf("Confirm() error = %v, want conflict", err)
	}
	if !strings.Contains(err.Error(), "does not belong to chapter") {
		t.Fatalf("Confirm() error = %v, want chapter mismatch conflict", err)
	}
}

func TestUseCaseRewriteRejectsMissingTargetText(t *testing.T) {
	projectRepo := memory.NewProjectRepository()
	assetRepo := memory.NewAssetRepository()
	chapterRepo := memory.NewChapterRepository()
	generationRepo := memory.NewGenerationRecordRepository()
	project := createProjectEntity(t, projectRepo, "11111111-1111-1111-1111-111111111111")
	chapter := createChapterEntity(t, chapterRepo, &chapterdomain.Chapter{
		ID:        "22222222-2222-2222-2222-222222222222",
		ProjectID: project.ID,
		Title:     "第一章",
		Ordinal:   1,
		Status:    chapterdomain.StatusDraft,
		Content:   "章节里没有这段文字。",
		CreatedAt: time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC),
	})

	useCase := NewUseCase(Dependencies{
		Chapters:          chapterRepo,
		Projects:          projectRepo,
		Assets:            assetRepo,
		GenerationRecords: generationRepo,
		PromptStore:       loadTestPromptStore(t),
		LLMClient:         &stubLLMClient{chatModel: &stubChatModel{}},
	})

	_, err := useCase.Rewrite(context.Background(), RewriteParams{ChapterID: chapter.ID, TargetText: "不存在的片段", Instruction: "改得更紧张。"})
	if !errors.Is(err, appservice.ErrInvalidInput) {
		t.Fatalf("Rewrite() error = %v, want invalid input", err)
	}
	if !strings.Contains(err.Error(), "target_text must exactly match") {
		t.Fatalf("Rewrite() error = %v, want target_text validation", err)
	}
	records, err := generationRepo.ListByChapter(context.Background(), generationdomain.ListByChapterParams{ChapterID: chapter.ID})
	if err != nil {
		t.Fatalf("ListByChapter(generation) error = %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("len(records) = %d, want 0", len(records))
	}
}

func TestUseCaseGenerateMarksRecordFailedOnEmptyLLMResponse(t *testing.T) {
	projectRepo := memory.NewProjectRepository()
	assetRepo := memory.NewAssetRepository()
	chapterRepo := memory.NewChapterRepository()
	generationRepo := memory.NewGenerationRecordRepository()
	project := createProjectEntity(t, projectRepo, "11111111-1111-1111-1111-111111111111")

	useCase := NewUseCase(Dependencies{
		Chapters:          chapterRepo,
		Projects:          projectRepo,
		Assets:            assetRepo,
		GenerationRecords: generationRepo,
		PromptStore:       loadTestPromptStore(t),
		LLMClient: &stubLLMClient{chatModel: &stubChatModel{generate: func(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
			return &schema.Message{Content: "   \n\t"}, nil
		}}},
	})

	_, err := useCase.Generate(context.Background(), GenerateParams{ProjectID: project.ID, Title: "第一章", Ordinal: 1, Instruction: "开始写。"})
	if err == nil || !strings.Contains(err.Error(), "llm response content must not be empty") {
		t.Fatalf("Generate() error = %v, want empty response error", err)
	}
	chapters, err := chapterRepo.ListByProject(context.Background(), chapterdomain.ListByProjectParams{ProjectID: project.ID})
	if err != nil {
		t.Fatalf("ListByProject(chapter) error = %v", err)
	}
	if len(chapters) != 0 {
		t.Fatalf("len(chapters) = %d, want 0", len(chapters))
	}
	records, err := generationRepo.ListByProject(context.Background(), generationdomain.ListByProjectParams{ProjectID: project.ID})
	if err != nil {
		t.Fatalf("ListByProject(generation) error = %v", err)
	}
	if len(records) != 1 || records[0].Status != generationdomain.StatusFailed {
		t.Fatalf("records = %#v, want single failed generation record", records)
	}
	if records[0].ErrorMessage == "" || records[0].OutputRef != "" {
		t.Fatalf("failed record = %#v, want error_message set and empty output_ref", records[0])
	}
}

func TestUseCaseErrorsOnNotFoundEntities(t *testing.T) {
	projectRepo := memory.NewProjectRepository()
	assetRepo := memory.NewAssetRepository()
	chapterRepo := memory.NewChapterRepository()
	generationRepo := memory.NewGenerationRecordRepository()
	useCase := NewUseCase(Dependencies{
		Chapters:          chapterRepo,
		Projects:          projectRepo,
		Assets:            assetRepo,
		GenerationRecords: generationRepo,
		PromptStore:       loadTestPromptStore(t),
		LLMClient:         &stubLLMClient{chatModel: &stubChatModel{}},
	})

	_, err := useCase.Generate(context.Background(), GenerateParams{
		ProjectID:   "11111111-1111-1111-1111-111111111111",
		Title:       "第一章",
		Ordinal:     1,
		Instruction: "开始写。",
	})
	if !errors.Is(err, appservice.ErrNotFound) {
		t.Fatalf("Generate() error = %v, want not found", err)
	}

	_, err = useCase.Continue(context.Background(), ContinueParams{
		ChapterID:   "22222222-2222-2222-2222-222222222222",
		Instruction: "继续写。",
	})
	if !errors.Is(err, appservice.ErrNotFound) {
		t.Fatalf("Continue() error = %v, want not found", err)
	}
}
