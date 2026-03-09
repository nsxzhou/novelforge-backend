package chapter

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	assetdomain "novelforge/backend/internal/domain/asset"
	chapterdomain "novelforge/backend/internal/domain/chapter"
	generationdomain "novelforge/backend/internal/domain/generation"
	projectdomain "novelforge/backend/internal/domain/project"
	"novelforge/backend/internal/infra/llm"
	"novelforge/backend/internal/infra/llm/prompts"
	appservice "novelforge/backend/internal/service"

	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

type useCase struct {
	chapters          chapterdomain.ChapterRepository
	projects          projectdomain.ProjectRepository
	assets            assetdomain.AssetRepository
	generationRecords generationdomain.GenerationRecordRepository
	llmClient         llm.Client
	promptStore       *prompts.Store
}

// NewUseCase 创建章节(chapter)用例实现。
func NewUseCase(deps Dependencies) UseCase {
	return &useCase{
		chapters:          deps.Chapters,
		projects:          deps.Projects,
		assets:            deps.Assets,
		generationRecords: deps.GenerationRecords,
		llmClient:         deps.LLMClient,
		promptStore:       deps.PromptStore,
	}
}

func (u *useCase) Create(ctx context.Context, entity *chapterdomain.Chapter) error {
	if entity == nil {
		return appservice.WrapInvalidInput(fmt.Errorf("chapter must not be nil"))
	}

	entity.ProjectID = strings.TrimSpace(entity.ProjectID)
	entity.Title = strings.TrimSpace(entity.Title)
	entity.Status = strings.TrimSpace(entity.Status)
	entity.Content = strings.TrimSpace(entity.Content)
	entity.CurrentDraftID = strings.TrimSpace(entity.CurrentDraftID)
	entity.CurrentDraftConfirmedBy = strings.TrimSpace(entity.CurrentDraftConfirmedBy)
	if entity.Status == "" {
		entity.Status = chapterdomain.StatusDraft
	}
	if err := validateChapterProjectID(entity.ProjectID); err != nil {
		return err
	}
	if err := u.ensureProjectExists(ctx, entity.ProjectID); err != nil {
		return err
	}
	if err := u.ensureOrdinalAvailable(ctx, entity.ProjectID, entity.Ordinal, ""); err != nil {
		return err
	}

	now := time.Now().UTC()
	entity.ID = uuid.NewString()
	entity.CreatedAt = now
	entity.UpdatedAt = now

	if err := entity.Validate(); err != nil {
		return appservice.WrapInvalidInput(err)
	}
	if err := u.chapters.Create(ctx, entity); err != nil {
		return appservice.TranslateStorageError(err)
	}
	return nil
}

func (u *useCase) GetByID(ctx context.Context, id string) (*chapterdomain.Chapter, error) {
	if err := validateChapterID(id); err != nil {
		return nil, err
	}

	entity, err := u.chapters.GetByID(ctx, strings.TrimSpace(id))
	if err != nil {
		return nil, appservice.TranslateStorageError(err)
	}
	return entity, nil
}

func (u *useCase) ListByProject(ctx context.Context, params chapterdomain.ListByProjectParams) ([]*chapterdomain.Chapter, error) {
	params.ProjectID = strings.TrimSpace(params.ProjectID)
	if err := validateChapterProjectID(params.ProjectID); err != nil {
		return nil, err
	}
	if err := u.ensureProjectExists(ctx, params.ProjectID); err != nil {
		return nil, err
	}

	items, err := u.chapters.ListByProject(ctx, params)
	if err != nil {
		return nil, appservice.TranslateStorageError(err)
	}
	return items, nil
}

func (u *useCase) Update(ctx context.Context, entity *chapterdomain.Chapter) error {
	if entity == nil {
		return appservice.WrapInvalidInput(fmt.Errorf("chapter must not be nil"))
	}
	if err := validateChapterID(entity.ID); err != nil {
		return err
	}

	existing, err := u.chapters.GetByID(ctx, strings.TrimSpace(entity.ID))
	if err != nil {
		return appservice.TranslateStorageError(err)
	}
	if err := u.ensureProjectExists(ctx, existing.ProjectID); err != nil {
		return err
	}
	if err := u.ensureOrdinalAvailable(ctx, existing.ProjectID, entity.Ordinal, existing.ID); err != nil {
		return err
	}

	entity.ID = existing.ID
	entity.ProjectID = existing.ProjectID
	entity.CreatedAt = existing.CreatedAt
	entity.Title = strings.TrimSpace(entity.Title)
	entity.Status = strings.TrimSpace(entity.Status)
	entity.Content = strings.TrimSpace(entity.Content)
	entity.CurrentDraftID = existing.CurrentDraftID
	entity.CurrentDraftConfirmedAt = existing.CurrentDraftConfirmedAt
	entity.CurrentDraftConfirmedBy = existing.CurrentDraftConfirmedBy
	entity.UpdatedAt = time.Now().UTC()

	if err := entity.Validate(); err != nil {
		return appservice.WrapInvalidInput(err)
	}
	if err := u.chapters.Update(ctx, entity); err != nil {
		return appservice.TranslateStorageError(err)
	}
	return nil
}

func (u *useCase) Generate(ctx context.Context, params GenerateParams) (*GenerateResult, error) {
	params.ProjectID = strings.TrimSpace(params.ProjectID)
	params.Title = strings.TrimSpace(params.Title)
	params.Instruction = strings.TrimSpace(params.Instruction)

	if err := validateChapterProjectID(params.ProjectID); err != nil {
		return nil, err
	}
	if params.Title == "" {
		return nil, appservice.WrapInvalidInput(fmt.Errorf("title must not be empty"))
	}
	if params.Ordinal <= 0 {
		return nil, appservice.WrapInvalidInput(fmt.Errorf("ordinal must be greater than 0"))
	}
	if params.Instruction == "" {
		return nil, appservice.WrapInvalidInput(fmt.Errorf("instruction must not be empty"))
	}

	projectEntity, promptContext, err := u.loadProjectAndPromptContext(ctx, params.ProjectID)
	if err != nil {
		return nil, err
	}
	if err := u.ensureOrdinalAvailable(ctx, params.ProjectID, params.Ordinal, ""); err != nil {
		return nil, err
	}

	systemPrompt, userPrompt, err := u.renderPrompt(generationdomain.KindChapterGeneration, buildGeneratePromptData(projectEntity, promptContext, params))
	if err != nil {
		return nil, err
	}

	chapterID := uuid.NewString()
	recordID := uuid.NewString()
	startedAt := time.Now().UTC()
	record := newGenerationRecord(recordID, params.ProjectID, chapterID, generationdomain.KindChapterGeneration, buildPromptSnapshot(systemPrompt, userPrompt), startedAt)
	if err := u.generationRecords.Create(ctx, record); err != nil {
		return nil, appservice.TranslateStorageError(err)
	}

	content, err := u.generateContent(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, u.failGeneration(ctx, record, "", startedAt, err)
	}

	now := time.Now().UTC()
	chapterEntity := &chapterdomain.Chapter{
		ID:                      chapterID,
		ProjectID:               params.ProjectID,
		Title:                   params.Title,
		Ordinal:                 params.Ordinal,
		Status:                  chapterdomain.StatusDraft,
		Content:                 content,
		CurrentDraftID:          recordID,
		CurrentDraftConfirmedAt: nil,
		CurrentDraftConfirmedBy: "",
		CreatedAt:               now,
		UpdatedAt:               now,
	}
	if err := chapterEntity.Validate(); err != nil {
		return nil, u.failGeneration(ctx, record, content, startedAt, appservice.WrapInvalidInput(err))
	}
	if err := u.chapters.Create(ctx, chapterEntity); err != nil {
		return nil, u.failGeneration(ctx, record, content, startedAt, appservice.TranslateStorageError(err))
	}
	if err := u.succeedGeneration(ctx, record, content, startedAt); err != nil {
		return nil, err
	}

	return &GenerateResult{Chapter: chapterEntity, GenerationRecord: record}, nil
}

func (u *useCase) Continue(ctx context.Context, params ContinueParams) (*ContinueResult, error) {
	params.ChapterID = strings.TrimSpace(params.ChapterID)
	params.Instruction = strings.TrimSpace(params.Instruction)

	if err := validateChapterID(params.ChapterID); err != nil {
		return nil, err
	}
	if params.Instruction == "" {
		return nil, appservice.WrapInvalidInput(fmt.Errorf("instruction must not be empty"))
	}

	chapterEntity, err := u.chapters.GetByID(ctx, params.ChapterID)
	if err != nil {
		return nil, appservice.TranslateStorageError(err)
	}
	projectEntity, promptContext, err := u.loadProjectAndPromptContext(ctx, chapterEntity.ProjectID)
	if err != nil {
		return nil, err
	}

	systemPrompt, userPrompt, err := u.renderPrompt(generationdomain.KindChapterContinuation, buildContinuePromptData(projectEntity, promptContext, chapterEntity, params))
	if err != nil {
		return nil, err
	}

	recordID := uuid.NewString()
	startedAt := time.Now().UTC()
	record := newGenerationRecord(recordID, chapterEntity.ProjectID, chapterEntity.ID, generationdomain.KindChapterContinuation, buildPromptSnapshot(systemPrompt, userPrompt), startedAt)
	if err := u.generationRecords.Create(ctx, record); err != nil {
		return nil, appservice.TranslateStorageError(err)
	}

	content, err := u.generateContent(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, u.failGeneration(ctx, record, "", startedAt, err)
	}

	updated := *chapterEntity
	updated.Status = chapterdomain.StatusDraft
	updated.Content = content
	updated.CurrentDraftID = recordID
	updated.CurrentDraftConfirmedAt = nil
	updated.CurrentDraftConfirmedBy = ""
	updated.UpdatedAt = time.Now().UTC()
	if err := updated.Validate(); err != nil {
		return nil, u.failGeneration(ctx, record, content, startedAt, appservice.WrapInvalidInput(err))
	}
	if err := u.chapters.Update(ctx, &updated); err != nil {
		return nil, u.failGeneration(ctx, record, content, startedAt, appservice.TranslateStorageError(err))
	}
	if err := u.succeedGeneration(ctx, record, content, startedAt); err != nil {
		return nil, err
	}

	return &ContinueResult{Chapter: &updated, GenerationRecord: record}, nil
}

func (u *useCase) Rewrite(ctx context.Context, params RewriteParams) (*RewriteResult, error) {
	params.ChapterID = strings.TrimSpace(params.ChapterID)
	params.Instruction = strings.TrimSpace(params.Instruction)
	trimmedTargetText := strings.TrimSpace(params.TargetText)

	if err := validateChapterID(params.ChapterID); err != nil {
		return nil, err
	}
	if params.Instruction == "" {
		return nil, appservice.WrapInvalidInput(fmt.Errorf("instruction must not be empty"))
	}
	if trimmedTargetText == "" {
		return nil, appservice.WrapInvalidInput(fmt.Errorf("target_text must not be empty"))
	}

	chapterEntity, err := u.chapters.GetByID(ctx, params.ChapterID)
	if err != nil {
		return nil, appservice.TranslateStorageError(err)
	}
	if !strings.Contains(chapterEntity.Content, params.TargetText) {
		return nil, appservice.WrapInvalidInput(fmt.Errorf("target_text must exactly match existing chapter content"))
	}

	projectEntity, promptContext, err := u.loadProjectAndPromptContext(ctx, chapterEntity.ProjectID)
	if err != nil {
		return nil, err
	}

	systemPrompt, userPrompt, err := u.renderPrompt(generationdomain.KindChapterRewrite, buildRewritePromptData(projectEntity, promptContext, chapterEntity, params))
	if err != nil {
		return nil, err
	}

	recordID := uuid.NewString()
	startedAt := time.Now().UTC()
	record := newGenerationRecord(recordID, chapterEntity.ProjectID, chapterEntity.ID, generationdomain.KindChapterRewrite, buildPromptSnapshot(systemPrompt, userPrompt), startedAt)
	if err := u.generationRecords.Create(ctx, record); err != nil {
		return nil, appservice.TranslateStorageError(err)
	}

	content, err := u.generateContent(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, u.failGeneration(ctx, record, "", startedAt, err)
	}

	updated := *chapterEntity
	updated.Status = chapterdomain.StatusDraft
	updated.Content = content
	updated.CurrentDraftID = recordID
	updated.CurrentDraftConfirmedAt = nil
	updated.CurrentDraftConfirmedBy = ""
	updated.UpdatedAt = time.Now().UTC()
	if err := updated.Validate(); err != nil {
		return nil, u.failGeneration(ctx, record, content, startedAt, appservice.WrapInvalidInput(err))
	}
	if err := u.chapters.Update(ctx, &updated); err != nil {
		return nil, u.failGeneration(ctx, record, content, startedAt, appservice.TranslateStorageError(err))
	}
	if err := u.succeedGeneration(ctx, record, content, startedAt); err != nil {
		return nil, err
	}

	return &RewriteResult{Chapter: &updated, GenerationRecord: record}, nil
}

func (u *useCase) ensureProjectExists(ctx context.Context, projectID string) error {
	if _, err := u.projects.GetByID(ctx, projectID); err != nil {
		return appservice.TranslateStorageError(err)
	}
	return nil
}

func (u *useCase) ensureOrdinalAvailable(ctx context.Context, projectID string, ordinal int, excludeChapterID string) error {
	items, err := u.chapters.ListByProject(ctx, chapterdomain.ListByProjectParams{ProjectID: projectID})
	if err != nil {
		return appservice.TranslateStorageError(err)
	}
	for _, item := range items {
		if item.Ordinal == ordinal && item.ID != excludeChapterID {
			return appservice.WrapInvalidInput(fmt.Errorf("ordinal already exists in project"))
		}
	}
	return nil
}

type chapterPromptContext struct {
	OutlineContext       string
	WorldbuildingContext string
	CharacterContext     string
}

func (u *useCase) loadProjectAndPromptContext(ctx context.Context, projectID string) (*projectdomain.Project, chapterPromptContext, error) {
	projectEntity, err := u.projects.GetByID(ctx, projectID)
	if err != nil {
		return nil, chapterPromptContext{}, appservice.TranslateStorageError(err)
	}

	assets, err := u.assets.ListByProject(ctx, assetdomain.ListByProjectParams{ProjectID: projectID})
	if err != nil {
		return nil, chapterPromptContext{}, appservice.TranslateStorageError(err)
	}

	outlineAssets := make([]*assetdomain.Asset, 0)
	worldbuildingAssets := make([]*assetdomain.Asset, 0)
	characterAssets := make([]*assetdomain.Asset, 0)
	for _, asset := range assets {
		switch asset.Type {
		case assetdomain.TypeOutline:
			outlineAssets = append(outlineAssets, asset)
		case assetdomain.TypeWorldbuilding:
			worldbuildingAssets = append(worldbuildingAssets, asset)
		case assetdomain.TypeCharacter:
			characterAssets = append(characterAssets, asset)
		}
	}

	return projectEntity, chapterPromptContext{
		OutlineContext:       formatAssetContext(outlineAssets),
		WorldbuildingContext: formatAssetContext(worldbuildingAssets),
		CharacterContext:     formatAssetContext(characterAssets),
	}, nil
}

func formatAssetContext(assets []*assetdomain.Asset) string {
	if len(assets) == 0 {
		return "（无）"
	}

	var builder strings.Builder
	for index, asset := range assets {
		if index > 0 {
			builder.WriteString("\n\n")
		}
		builder.WriteString("标题：")
		builder.WriteString(strings.TrimSpace(asset.Title))
		builder.WriteString("\n内容：\n")
		builder.WriteString(strings.TrimSpace(asset.Content))
	}
	return builder.String()
}

func buildGeneratePromptData(projectEntity *projectdomain.Project, promptContext chapterPromptContext, params GenerateParams) map[string]string {
	return map[string]string{
		"ProjectTitle":         projectEntity.Title,
		"ProjectSummary":       projectEntity.Summary,
		"ChapterTitle":         params.Title,
		"ChapterOrdinal":       strconv.Itoa(params.Ordinal),
		"OutlineContext":       promptContext.OutlineContext,
		"WorldbuildingContext": promptContext.WorldbuildingContext,
		"CharacterContext":     promptContext.CharacterContext,
		"Instruction":          params.Instruction,
	}
}

func buildContinuePromptData(projectEntity *projectdomain.Project, promptContext chapterPromptContext, chapterEntity *chapterdomain.Chapter, params ContinueParams) map[string]string {
	return map[string]string{
		"ProjectTitle":         projectEntity.Title,
		"ProjectSummary":       projectEntity.Summary,
		"ChapterTitle":         chapterEntity.Title,
		"ChapterOrdinal":       strconv.Itoa(chapterEntity.Ordinal),
		"OutlineContext":       promptContext.OutlineContext,
		"WorldbuildingContext": promptContext.WorldbuildingContext,
		"CharacterContext":     promptContext.CharacterContext,
		"CurrentChapterContent": chapterEntity.Content,
		"Instruction":           params.Instruction,
	}
}

func buildRewritePromptData(projectEntity *projectdomain.Project, promptContext chapterPromptContext, chapterEntity *chapterdomain.Chapter, params RewriteParams) map[string]string {
	return map[string]string{
		"ProjectTitle":         projectEntity.Title,
		"ProjectSummary":       projectEntity.Summary,
		"ChapterTitle":         chapterEntity.Title,
		"ChapterOrdinal":       strconv.Itoa(chapterEntity.Ordinal),
		"OutlineContext":       promptContext.OutlineContext,
		"WorldbuildingContext": promptContext.WorldbuildingContext,
		"CharacterContext":     promptContext.CharacterContext,
		"CurrentChapterContent": chapterEntity.Content,
		"TargetText":           params.TargetText,
		"Instruction":          params.Instruction,
	}
}

func (u *useCase) renderPrompt(kind string, promptData map[string]string) (string, string, error) {
	if u.promptStore == nil {
		return "", "", fmt.Errorf("prompt store is not configured")
	}
	template, ok := u.promptStore.Get(kind)
	if !ok {
		return "", "", fmt.Errorf("prompt template %q not found", kind)
	}

	systemPrompt, userPrompt, err := template.Render(promptData)
	if err != nil {
		return "", "", fmt.Errorf("render prompt template %q: %w", kind, err)
	}
	return systemPrompt, userPrompt, nil
}

func (u *useCase) generateContent(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if u.llmClient == nil || u.llmClient.ChatModel() == nil {
		return "", fmt.Errorf("llm client is not configured")
	}

	response, err := u.llmClient.ChatModel().Generate(ctx, []*schema.Message{
		{Role: schema.System, Content: systemPrompt},
		{Role: schema.User, Content: userPrompt},
	})
	if err != nil {
		return "", fmt.Errorf("generate chapter content: %w", err)
	}
	if response == nil || strings.TrimSpace(response.Content) == "" {
		return "", fmt.Errorf("llm response content must not be empty")
	}
	return strings.TrimSpace(response.Content), nil
}

func buildPromptSnapshot(systemPrompt, userPrompt string) string {
	return fmt.Sprintf("system:\n%s\n\nuser:\n%s", strings.TrimSpace(systemPrompt), strings.TrimSpace(userPrompt))
}

func (u *useCase) succeedGeneration(ctx context.Context, record *generationdomain.GenerationRecord, outputRef string, startedAt time.Time) error {
	updatedAt := time.Now().UTC()
	params := generationdomain.UpdateStatusParams{
		ID:             record.ID,
		Status:         generationdomain.StatusSucceeded,
		OutputRef:      outputRef,
		TokenUsage:     0,
		DurationMillis: durationMillis(startedAt, updatedAt),
		ErrorMessage:   "",
		UpdatedAt:      updatedAt,
	}
	if err := u.generationRecords.UpdateStatus(ctx, params); err != nil {
		return appservice.TranslateStorageError(err)
	}
	record.Status = params.Status
	record.OutputRef = params.OutputRef
	record.TokenUsage = params.TokenUsage
	record.DurationMillis = params.DurationMillis
	record.ErrorMessage = params.ErrorMessage
	record.UpdatedAt = params.UpdatedAt
	return nil
}

func (u *useCase) failGeneration(ctx context.Context, record *generationdomain.GenerationRecord, outputRef string, startedAt time.Time, cause error) error {
	updatedAt := time.Now().UTC()
	params := generationdomain.UpdateStatusParams{
		ID:             record.ID,
		Status:         generationdomain.StatusFailed,
		OutputRef:      outputRef,
		TokenUsage:     0,
		DurationMillis: durationMillis(startedAt, updatedAt),
		ErrorMessage:   cause.Error(),
		UpdatedAt:      updatedAt,
	}
	if err := u.generationRecords.UpdateStatus(ctx, params); err != nil {
		return fmt.Errorf("mark generation failed: %w; original error: %v", appservice.TranslateStorageError(err), cause)
	}
	record.Status = params.Status
	record.OutputRef = params.OutputRef
	record.TokenUsage = params.TokenUsage
	record.DurationMillis = params.DurationMillis
	record.ErrorMessage = params.ErrorMessage
	record.UpdatedAt = params.UpdatedAt
	return cause
}

func durationMillis(startedAt, endedAt time.Time) int64 {
	if endedAt.Before(startedAt) {
		return 0
	}
	return endedAt.Sub(startedAt).Milliseconds()
}

func newGenerationRecord(id, projectID, chapterID, kind, inputSnapshot string, now time.Time) *generationdomain.GenerationRecord {
	return &generationdomain.GenerationRecord{
		ID:               id,
		ProjectID:        projectID,
		ChapterID:        chapterID,
		ConversationID:   "",
		Kind:             kind,
		Status:           generationdomain.StatusRunning,
		InputSnapshotRef: inputSnapshot,
		OutputRef:        "",
		TokenUsage:       0,
		DurationMillis:   0,
		ErrorMessage:     "",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

func validateChapterID(id string) error {
	if _, err := uuid.Parse(strings.TrimSpace(id)); err != nil {
		return appservice.WrapInvalidInput(fmt.Errorf("chapter_id must be a valid UUID"))
	}
	return nil
}

func validateChapterProjectID(projectID string) error {
	if _, err := uuid.Parse(strings.TrimSpace(projectID)); err != nil {
		return appservice.WrapInvalidInput(fmt.Errorf("project_id must be a valid UUID"))
	}
	return nil
}
