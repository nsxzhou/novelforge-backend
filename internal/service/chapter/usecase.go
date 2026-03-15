package chapter

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	assetdomain "novelforge/backend/internal/domain/asset"
	chapterdomain "novelforge/backend/internal/domain/chapter"
	generationdomain "novelforge/backend/internal/domain/generation"
	metricdomain "novelforge/backend/internal/domain/metric"
	projectdomain "novelforge/backend/internal/domain/project"
	promptdomain "novelforge/backend/internal/domain/prompt"
	"novelforge/backend/internal/infra/llm"
	"novelforge/backend/internal/infra/llm/prompts"
	appservice "novelforge/backend/internal/service"
	metricservice "novelforge/backend/internal/service/metric"

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
	promptOverrides   promptdomain.OverrideRepository
	metrics           metricservice.UseCase
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
		promptOverrides:   deps.PromptOverrides,
		metrics:           deps.Metrics,
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
	startedAt := time.Now().UTC()
	params.ProjectID = strings.TrimSpace(params.ProjectID)
	params.Title = strings.TrimSpace(params.Title)
	params.Instruction = strings.TrimSpace(params.Instruction)

	if err := validateChapterProjectID(params.ProjectID); err != nil {
		return nil, err
	}
	if params.Title == "" {
		err := appservice.WrapInvalidInput(fmt.Errorf("title must not be empty"))
		u.trackOperationFailed(ctx, params.ProjectID, "", "generate", generationdomain.KindChapterGeneration, startedAt, 0, err)
		return nil, err
	}
	if params.Ordinal <= 0 {
		err := appservice.WrapInvalidInput(fmt.Errorf("ordinal must be greater than 0"))
		u.trackOperationFailed(ctx, params.ProjectID, "", "generate", generationdomain.KindChapterGeneration, startedAt, 0, err)
		return nil, err
	}
	if params.Instruction == "" {
		err := appservice.WrapInvalidInput(fmt.Errorf("instruction must not be empty"))
		u.trackOperationFailed(ctx, params.ProjectID, "", "generate", generationdomain.KindChapterGeneration, startedAt, 0, err)
		return nil, err
	}

	projectEntity, promptContext, err := u.loadProjectAndPromptContext(ctx, params.ProjectID)
	if err != nil {
		u.trackOperationFailed(ctx, params.ProjectID, "", "generate", generationdomain.KindChapterGeneration, startedAt, 0, err)
		return nil, err
	}
	if err := u.ensureOrdinalAvailable(ctx, params.ProjectID, params.Ordinal, ""); err != nil {
		u.trackOperationFailed(ctx, params.ProjectID, "", "generate", generationdomain.KindChapterGeneration, startedAt, 0, err)
		return nil, err
	}

	systemPrompt, userPrompt, err := u.renderPrompt(ctx, params.ProjectID, generationdomain.KindChapterGeneration, buildGeneratePromptData(projectEntity, promptContext, params))
	if err != nil {
		u.trackOperationFailed(ctx, params.ProjectID, "", "generate", generationdomain.KindChapterGeneration, startedAt, 0, err)
		return nil, err
	}

	chapterID := uuid.NewString()
	recordID := uuid.NewString()
	record := newGenerationRecord(recordID, params.ProjectID, chapterID, generationdomain.KindChapterGeneration, buildPromptSnapshot(systemPrompt, userPrompt), startedAt)
	if err := u.generationRecords.Create(ctx, record); err != nil {
		translatedErr := appservice.TranslateStorageError(err)
		u.trackOperationFailed(ctx, params.ProjectID, chapterID, "generate", generationdomain.KindChapterGeneration, startedAt, 0, translatedErr)
		return nil, translatedErr
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
	startedAt := time.Now().UTC()
	params.ChapterID = strings.TrimSpace(params.ChapterID)
	params.Instruction = strings.TrimSpace(params.Instruction)

	if err := validateChapterID(params.ChapterID); err != nil {
		u.trackOperationFailed(ctx, "", params.ChapterID, "continue", generationdomain.KindChapterContinuation, startedAt, 0, err)
		return nil, err
	}
	if params.Instruction == "" {
		err := appservice.WrapInvalidInput(fmt.Errorf("instruction must not be empty"))
		u.trackOperationFailed(ctx, "", params.ChapterID, "continue", generationdomain.KindChapterContinuation, startedAt, 0, err)
		return nil, err
	}

	chapterEntity, err := u.chapters.GetByID(ctx, params.ChapterID)
	if err != nil {
		translatedErr := appservice.TranslateStorageError(err)
		u.trackOperationFailed(ctx, "", params.ChapterID, "continue", generationdomain.KindChapterContinuation, startedAt, 0, translatedErr)
		return nil, translatedErr
	}
	expectedUpdatedAt := chapterEntity.UpdatedAt
	projectEntity, promptContext, err := u.loadProjectAndPromptContext(ctx, chapterEntity.ProjectID)
	if err != nil {
		u.trackOperationFailed(ctx, chapterEntity.ProjectID, chapterEntity.ID, "continue", generationdomain.KindChapterContinuation, startedAt, 0, err)
		return nil, err
	}

	systemPrompt, userPrompt, err := u.renderPrompt(ctx, chapterEntity.ProjectID, generationdomain.KindChapterContinuation, buildContinuePromptData(projectEntity, promptContext, chapterEntity, params))
	if err != nil {
		u.trackOperationFailed(ctx, chapterEntity.ProjectID, chapterEntity.ID, "continue", generationdomain.KindChapterContinuation, startedAt, 0, err)
		return nil, err
	}

	recordID := uuid.NewString()
	record := newGenerationRecord(recordID, chapterEntity.ProjectID, chapterEntity.ID, generationdomain.KindChapterContinuation, buildPromptSnapshot(systemPrompt, userPrompt), startedAt)
	if err := u.generationRecords.Create(ctx, record); err != nil {
		translatedErr := appservice.TranslateStorageError(err)
		u.trackOperationFailed(ctx, chapterEntity.ProjectID, chapterEntity.ID, "continue", generationdomain.KindChapterContinuation, startedAt, 0, translatedErr)
		return nil, translatedErr
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
	updatedOK, err := u.chapters.UpdateIfUnchanged(ctx, &updated, expectedUpdatedAt)
	if err != nil {
		return nil, u.failGeneration(ctx, record, content, startedAt, appservice.TranslateStorageError(err))
	}
	if !updatedOK {
		return nil, u.failGeneration(ctx, record, content, startedAt, appservice.WrapConflict(fmt.Errorf("chapter was modified during continuation; please retry")))
	}
	if err := u.succeedGeneration(ctx, record, content, startedAt); err != nil {
		return nil, err
	}

	return &ContinueResult{Chapter: &updated, GenerationRecord: record}, nil
}

func (u *useCase) Rewrite(ctx context.Context, params RewriteParams) (*RewriteResult, error) {
	startedAt := time.Now().UTC()
	params.ChapterID = strings.TrimSpace(params.ChapterID)
	params.Instruction = strings.TrimSpace(params.Instruction)
	trimmedTargetText := strings.TrimSpace(params.TargetText)

	if err := validateChapterID(params.ChapterID); err != nil {
		u.trackOperationFailed(ctx, "", params.ChapterID, "rewrite", generationdomain.KindChapterRewrite, startedAt, 0, err)
		return nil, err
	}
	if params.Instruction == "" {
		err := appservice.WrapInvalidInput(fmt.Errorf("instruction must not be empty"))
		u.trackOperationFailed(ctx, "", params.ChapterID, "rewrite", generationdomain.KindChapterRewrite, startedAt, 0, err)
		return nil, err
	}
	if trimmedTargetText == "" {
		err := appservice.WrapInvalidInput(fmt.Errorf("target_text must not be empty"))
		u.trackOperationFailed(ctx, "", params.ChapterID, "rewrite", generationdomain.KindChapterRewrite, startedAt, 0, err)
		return nil, err
	}

	chapterEntity, err := u.chapters.GetByID(ctx, params.ChapterID)
	if err != nil {
		translatedErr := appservice.TranslateStorageError(err)
		u.trackOperationFailed(ctx, "", params.ChapterID, "rewrite", generationdomain.KindChapterRewrite, startedAt, 0, translatedErr)
		return nil, translatedErr
	}
	expectedUpdatedAt := chapterEntity.UpdatedAt
	if !strings.Contains(chapterEntity.Content, trimmedTargetText) {
		err := appservice.WrapInvalidInput(fmt.Errorf("target_text must exactly match existing chapter content"))
		u.trackOperationFailed(ctx, chapterEntity.ProjectID, chapterEntity.ID, "rewrite", generationdomain.KindChapterRewrite, startedAt, 0, err)
		return nil, err
	}

	projectEntity, promptContext, err := u.loadProjectAndPromptContext(ctx, chapterEntity.ProjectID)
	if err != nil {
		u.trackOperationFailed(ctx, chapterEntity.ProjectID, chapterEntity.ID, "rewrite", generationdomain.KindChapterRewrite, startedAt, 0, err)
		return nil, err
	}

	systemPrompt, userPrompt, err := u.renderPrompt(ctx, chapterEntity.ProjectID, generationdomain.KindChapterRewrite, buildRewritePromptData(projectEntity, promptContext, chapterEntity, params))
	if err != nil {
		u.trackOperationFailed(ctx, chapterEntity.ProjectID, chapterEntity.ID, "rewrite", generationdomain.KindChapterRewrite, startedAt, 0, err)
		return nil, err
	}

	recordID := uuid.NewString()
	record := newGenerationRecord(recordID, chapterEntity.ProjectID, chapterEntity.ID, generationdomain.KindChapterRewrite, buildPromptSnapshot(systemPrompt, userPrompt), startedAt)
	if err := u.generationRecords.Create(ctx, record); err != nil {
		translatedErr := appservice.TranslateStorageError(err)
		u.trackOperationFailed(ctx, chapterEntity.ProjectID, chapterEntity.ID, "rewrite", generationdomain.KindChapterRewrite, startedAt, 0, translatedErr)
		return nil, translatedErr
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
	updatedOK, err := u.chapters.UpdateIfUnchanged(ctx, &updated, expectedUpdatedAt)
	if err != nil {
		return nil, u.failGeneration(ctx, record, content, startedAt, appservice.TranslateStorageError(err))
	}
	if !updatedOK {
		return nil, u.failGeneration(ctx, record, content, startedAt, appservice.WrapConflict(fmt.Errorf("chapter was modified during rewrite; please retry")))
	}
	if err := u.succeedGeneration(ctx, record, content, startedAt); err != nil {
		return nil, err
	}

	return &RewriteResult{Chapter: &updated, GenerationRecord: record}, nil
}

func (u *useCase) Confirm(ctx context.Context, params ConfirmParams) (*chapterdomain.Chapter, error) {
	startedAt := time.Now().UTC()
	params.ChapterID = strings.TrimSpace(params.ChapterID)
	params.ConfirmedBy = strings.TrimSpace(params.ConfirmedBy)

	if err := validateChapterID(params.ChapterID); err != nil {
		u.trackOperationFailed(ctx, "", params.ChapterID, "confirm", "", startedAt, 0, err)
		return nil, err
	}
	if err := validateConfirmedBy(params.ConfirmedBy); err != nil {
		u.trackOperationFailed(ctx, "", params.ChapterID, "confirm", "", startedAt, 0, err)
		return nil, err
	}

	chapterEntity, err := u.chapters.GetByID(ctx, params.ChapterID)
	if err != nil {
		translatedErr := appservice.TranslateStorageError(err)
		u.trackOperationFailed(ctx, "", params.ChapterID, "confirm", "", startedAt, 0, translatedErr)
		return nil, translatedErr
	}
	expectedUpdatedAt := chapterEntity.UpdatedAt
	if chapterEntity.CurrentDraftID == "" {
		err := appservice.WrapConflict(fmt.Errorf("current_draft_id must not be empty"))
		u.trackOperationFailed(ctx, chapterEntity.ProjectID, chapterEntity.ID, "confirm", "", startedAt, 0, err)
		return nil, err
	}
	if chapterEntity.Status == chapterdomain.StatusConfirmed {
		if chapterEntity.CurrentDraftConfirmedAt != nil && chapterEntity.CurrentDraftConfirmedBy != "" {
			u.trackOperationSucceeded(ctx, chapterEntity.ProjectID, chapterEntity.ID, "confirm", "", startedAt, 0)
			return chapterEntity, nil
		}
		err := appservice.WrapConflict(fmt.Errorf("chapter is already confirmed"))
		u.trackOperationFailed(ctx, chapterEntity.ProjectID, chapterEntity.ID, "confirm", "", startedAt, 0, err)
		return nil, err
	}

	record, err := u.generationRecords.GetByID(ctx, chapterEntity.CurrentDraftID)
	if err != nil {
		translatedErr := appservice.TranslateStorageError(err)
		u.trackOperationFailed(ctx, chapterEntity.ProjectID, chapterEntity.ID, "confirm", "", startedAt, 0, translatedErr)
		return nil, translatedErr
	}
	if record.ChapterID != chapterEntity.ID {
		err := appservice.WrapConflict(fmt.Errorf("current draft generation record does not belong to chapter"))
		u.trackOperationFailed(ctx, chapterEntity.ProjectID, chapterEntity.ID, "confirm", "", startedAt, 0, err)
		return nil, err
	}
	if record.Status != generationdomain.StatusSucceeded {
		err := appservice.WrapConflict(fmt.Errorf("current draft generation record must be succeeded before confirmation"))
		u.trackOperationFailed(ctx, chapterEntity.ProjectID, chapterEntity.ID, "confirm", "", startedAt, 0, err)
		return nil, err
	}

	now := time.Now().UTC()
	updated := *chapterEntity
	updated.Status = chapterdomain.StatusConfirmed
	updated.CurrentDraftConfirmedAt = &now
	updated.CurrentDraftConfirmedBy = params.ConfirmedBy
	updated.UpdatedAt = now
	if err := updated.Validate(); err != nil {
		wrappedErr := appservice.WrapInvalidInput(err)
		u.trackOperationFailed(ctx, chapterEntity.ProjectID, chapterEntity.ID, "confirm", "", startedAt, 0, wrappedErr)
		return nil, wrappedErr
	}
	updatedOK, err := u.chapters.UpdateIfUnchanged(ctx, &updated, expectedUpdatedAt)
	if err != nil {
		translatedErr := appservice.TranslateStorageError(err)
		u.trackOperationFailed(ctx, chapterEntity.ProjectID, chapterEntity.ID, "confirm", "", startedAt, 0, translatedErr)
		return nil, translatedErr
	}
	if !updatedOK {
		err := appservice.WrapConflict(fmt.Errorf("chapter was modified during confirmation; please retry"))
		u.trackOperationFailed(ctx, chapterEntity.ProjectID, chapterEntity.ID, "confirm", "", startedAt, 0, err)
		return nil, err
	}

	u.trackOperationSucceeded(ctx, updated.ProjectID, updated.ID, "confirm", "", startedAt, 0)
	return &updated, nil
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
		"ProjectTitle":          projectEntity.Title,
		"ProjectSummary":        projectEntity.Summary,
		"ChapterTitle":          chapterEntity.Title,
		"ChapterOrdinal":        strconv.Itoa(chapterEntity.Ordinal),
		"OutlineContext":        promptContext.OutlineContext,
		"WorldbuildingContext":  promptContext.WorldbuildingContext,
		"CharacterContext":      promptContext.CharacterContext,
		"CurrentChapterContent": chapterEntity.Content,
		"Instruction":           params.Instruction,
	}
}

func buildRewritePromptData(projectEntity *projectdomain.Project, promptContext chapterPromptContext, chapterEntity *chapterdomain.Chapter, params RewriteParams) map[string]string {
	trimmedTargetText := strings.TrimSpace(params.TargetText)
	return map[string]string{
		"ProjectTitle":          projectEntity.Title,
		"ProjectSummary":        projectEntity.Summary,
		"ChapterTitle":          chapterEntity.Title,
		"ChapterOrdinal":        strconv.Itoa(chapterEntity.Ordinal),
		"OutlineContext":        promptContext.OutlineContext,
		"WorldbuildingContext":  promptContext.WorldbuildingContext,
		"CharacterContext":      promptContext.CharacterContext,
		"CurrentChapterContent": chapterEntity.Content,
		"TargetText":            trimmedTargetText,
		"Instruction":           params.Instruction,
	}
}

func (u *useCase) renderPrompt(ctx context.Context, projectID string, kind string, promptData map[string]string) (string, string, error) {
	if u.promptStore == nil {
		return "", "", fmt.Errorf("prompt store is not configured")
	}
	promptCapability, ok := appservice.PromptCapabilityForGenerationKind(kind)
	if !ok {
		return "", "", fmt.Errorf("unsupported generation kind %q", kind)
	}

	// 优先查项目覆盖
	if u.promptOverrides != nil {
		override, err := u.promptOverrides.GetByProjectAndCapability(ctx, projectID, string(promptCapability))
		if err == nil && override != nil {
			tmpl, parseErr := prompts.ParseTemplate(string(promptCapability), override.System, override.User)
			if parseErr != nil {
				return "", "", fmt.Errorf("invalid project prompt override: %w", parseErr)
			}
			systemPrompt, userPrompt, renderErr := tmpl.Render(promptData)
			if renderErr != nil {
				return "", "", fmt.Errorf("render project prompt override %q: %w", promptCapability, renderErr)
			}
			return systemPrompt, userPrompt, nil
		}
	}

	// fallback 到全局默认
	template, ok := u.promptStore.Get(promptCapability)
	if !ok {
		return "", "", fmt.Errorf("prompt template %q not found", promptCapability)
	}

	systemPrompt, userPrompt, err := template.Render(promptData)
	if err != nil {
		return "", "", fmt.Errorf("render prompt template %q: %w", promptCapability, err)
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
	duration := durationMillis(startedAt, updatedAt)
	params := generationdomain.UpdateStatusParams{
		ID:             record.ID,
		Status:         generationdomain.StatusSucceeded,
		OutputRef:      outputRef,
		TokenUsage:     0,
		DurationMillis: duration,
		ErrorMessage:   "",
		UpdatedAt:      updatedAt,
	}
	if err := u.generationRecords.UpdateStatus(ctx, params); err != nil {
		translatedErr := appservice.TranslateStorageError(err)
		u.trackOperationFailed(ctx, record.ProjectID, record.ChapterID, operationActionForGenerationKind(record.Kind), record.Kind, startedAt, 0, translatedErr)
		return translatedErr
	}
	record.Status = params.Status
	record.OutputRef = params.OutputRef
	record.TokenUsage = params.TokenUsage
	record.DurationMillis = params.DurationMillis
	record.ErrorMessage = params.ErrorMessage
	record.UpdatedAt = params.UpdatedAt
	u.trackOperationSucceeded(ctx, record.ProjectID, record.ChapterID, operationActionForGenerationKind(record.Kind), record.Kind, startedAt, record.TokenUsage)
	return nil
}

func (u *useCase) failGeneration(ctx context.Context, record *generationdomain.GenerationRecord, outputRef string, startedAt time.Time, cause error) error {
	updatedAt := time.Now().UTC()
	duration := durationMillis(startedAt, updatedAt)
	params := generationdomain.UpdateStatusParams{
		ID:             record.ID,
		Status:         generationdomain.StatusFailed,
		OutputRef:      outputRef,
		TokenUsage:     0,
		DurationMillis: duration,
		ErrorMessage:   cause.Error(),
		UpdatedAt:      updatedAt,
	}
	if err := u.generationRecords.UpdateStatus(ctx, params); err != nil {
		wrappedErr := fmt.Errorf("mark generation failed: %w; original error: %v", appservice.TranslateStorageError(err), cause)
		u.trackOperationFailed(ctx, record.ProjectID, record.ChapterID, operationActionForGenerationKind(record.Kind), record.Kind, startedAt, 0, wrappedErr)
		return wrappedErr
	}
	record.Status = params.Status
	record.OutputRef = params.OutputRef
	record.TokenUsage = params.TokenUsage
	record.DurationMillis = params.DurationMillis
	record.ErrorMessage = params.ErrorMessage
	record.UpdatedAt = params.UpdatedAt
	u.trackOperationFailed(ctx, record.ProjectID, record.ChapterID, operationActionForGenerationKind(record.Kind), record.Kind, startedAt, record.TokenUsage, cause)
	return cause
}

func operationActionForGenerationKind(kind string) string {
	switch kind {
	case generationdomain.KindChapterGeneration:
		return "generate"
	case generationdomain.KindChapterContinuation:
		return "continue"
	case generationdomain.KindChapterRewrite:
		return "rewrite"
	default:
		return ""
	}
}

func (u *useCase) trackOperationSucceeded(ctx context.Context, projectID, chapterID, action, generationKind string, startedAt time.Time, tokenUsage int) {
	if strings.TrimSpace(action) == "" {
		return
	}
	u.appendMetricEvent(ctx, metricdomain.EventOperationCompleted, projectID, chapterID, action, generationKind, tokenUsage, startedAt, nil)
}

func (u *useCase) trackOperationFailed(ctx context.Context, projectID, chapterID, action, generationKind string, startedAt time.Time, tokenUsage int, cause error) {
	if strings.TrimSpace(action) == "" {
		return
	}
	u.appendMetricEvent(ctx, metricdomain.EventOperationFailed, projectID, chapterID, action, generationKind, tokenUsage, startedAt, cause)
}

func (u *useCase) appendMetricEvent(ctx context.Context, eventName, projectID, chapterID, action, generationKind string, tokenUsage int, startedAt time.Time, cause error) {
	if u.metrics == nil {
		return
	}

	projectID = strings.TrimSpace(projectID)
	if _, err := uuid.Parse(projectID); err != nil {
		// 无法确定项目归属时跳过落库，避免污染事件表。
		log.Printf("metric append skipped event_name=%s action=%s reason=invalid_project_id", eventName, action)
		return
	}

	chapterID = strings.TrimSpace(chapterID)
	if chapterID != "" {
		if _, err := uuid.Parse(chapterID); err != nil {
			chapterID = ""
		}
	}

	labels := map[string]string{
		"domain": "chapter",
		"action": action,
	}
	if generationKind != "" {
		labels["generation_kind"] = generationKind
	}
	if cause != nil {
		labels["error_kind"] = errorKindFor(cause)
	}

	event := &metricdomain.MetricEvent{
		EventName: eventName,
		ProjectID: projectID,
		ChapterID: chapterID,
		Labels:    labels,
		Stats: map[string]float64{
			"duration_ms": float64(durationMillis(startedAt, time.Now().UTC())),
			"token_usage": float64(tokenUsage),
		},
		OccurredAt: time.Now().UTC(),
	}
	if err := u.metrics.Append(ctx, event); err != nil {
		// 埋点失败不影响主业务流程，仅记录 warning 以便排查。
		log.Printf("metric append failed event_name=%s action=%s project_id=%s chapter_id=%s err=%v", eventName, action, projectID, chapterID, err)
	}
}

func errorKindFor(err error) string {
	switch {
	case errors.Is(err, appservice.ErrInvalidInput):
		return "invalid_input"
	case errors.Is(err, appservice.ErrNotFound):
		return "not_found"
	case errors.Is(err, appservice.ErrConflict):
		return "conflict"
	default:
		return "internal"
	}
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

func validateConfirmedBy(confirmedBy string) error {
	if _, err := uuid.Parse(strings.TrimSpace(confirmedBy)); err != nil {
		return appservice.WrapInvalidInput(fmt.Errorf("confirmed_by must be a valid UUID"))
	}
	return nil
}

func (u *useCase) streamContent(ctx context.Context, systemPrompt, userPrompt string) (*schema.StreamReader[*schema.Message], error) {
	if u.llmClient == nil || u.llmClient.ChatModel() == nil {
		return nil, fmt.Errorf("llm client is not configured")
	}

	stream, err := u.llmClient.ChatModel().Stream(ctx, []*schema.Message{
		{Role: schema.System, Content: systemPrompt},
		{Role: schema.User, Content: userPrompt},
	})
	if err != nil {
		return nil, fmt.Errorf("stream chapter content: %w", err)
	}
	return stream, nil
}

func (u *useCase) GenerateStream(ctx context.Context, params GenerateParams) (*GenerateStreamResult, error) {
	startedAt := time.Now().UTC()
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

	systemPrompt, userPrompt, err := u.renderPrompt(ctx, params.ProjectID, generationdomain.KindChapterGeneration, buildGeneratePromptData(projectEntity, promptContext, params))
	if err != nil {
		return nil, err
	}

	chapterID := uuid.NewString()
	recordID := uuid.NewString()
	record := newGenerationRecord(recordID, params.ProjectID, chapterID, generationdomain.KindChapterGeneration, buildPromptSnapshot(systemPrompt, userPrompt), startedAt)
	if err := u.generationRecords.Create(ctx, record); err != nil {
		return nil, appservice.TranslateStorageError(err)
	}

	stream, err := u.streamContent(ctx, systemPrompt, userPrompt)
	if err != nil {
		_ = u.failGeneration(ctx, record, "", startedAt, err)
		return nil, err
	}

	return &GenerateStreamResult{
		ChapterID: chapterID,
		RecordID:  recordID,
		Record:    record,
		Stream:    stream,
		OnComplete: func(content string) (*GenerateResult, error) {
			content = strings.TrimSpace(content)
			if content == "" {
				err := fmt.Errorf("llm response content must not be empty")
				_ = u.failGeneration(ctx, record, "", startedAt, err)
				return nil, err
			}

			now := time.Now().UTC()
			chapterEntity := &chapterdomain.Chapter{
				ID:             chapterID,
				ProjectID:      params.ProjectID,
				Title:          params.Title,
				Ordinal:        params.Ordinal,
				Status:         chapterdomain.StatusDraft,
				Content:        content,
				CurrentDraftID: recordID,
				CreatedAt:      now,
				UpdatedAt:      now,
			}
			if err := chapterEntity.Validate(); err != nil {
				_ = u.failGeneration(ctx, record, content, startedAt, appservice.WrapInvalidInput(err))
				return nil, appservice.WrapInvalidInput(err)
			}
			if err := u.chapters.Create(ctx, chapterEntity); err != nil {
				translatedErr := appservice.TranslateStorageError(err)
				_ = u.failGeneration(ctx, record, content, startedAt, translatedErr)
				return nil, translatedErr
			}
			if err := u.succeedGeneration(ctx, record, content, startedAt); err != nil {
				return nil, err
			}
			return &GenerateResult{Chapter: chapterEntity, GenerationRecord: record}, nil
		},
		OnError: func(err error) {
			_ = u.failGeneration(ctx, record, "", startedAt, err)
		},
	}, nil
}

func (u *useCase) ContinueStream(ctx context.Context, params ContinueParams) (*ContinueStreamResult, error) {
	startedAt := time.Now().UTC()
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
	expectedUpdatedAt := chapterEntity.UpdatedAt
	projectEntity, promptContext, err := u.loadProjectAndPromptContext(ctx, chapterEntity.ProjectID)
	if err != nil {
		return nil, err
	}

	systemPrompt, userPrompt, err := u.renderPrompt(ctx, chapterEntity.ProjectID, generationdomain.KindChapterContinuation, buildContinuePromptData(projectEntity, promptContext, chapterEntity, params))
	if err != nil {
		return nil, err
	}

	recordID := uuid.NewString()
	record := newGenerationRecord(recordID, chapterEntity.ProjectID, chapterEntity.ID, generationdomain.KindChapterContinuation, buildPromptSnapshot(systemPrompt, userPrompt), startedAt)
	if err := u.generationRecords.Create(ctx, record); err != nil {
		return nil, appservice.TranslateStorageError(err)
	}

	stream, err := u.streamContent(ctx, systemPrompt, userPrompt)
	if err != nil {
		_ = u.failGeneration(ctx, record, "", startedAt, err)
		return nil, err
	}

	return &ContinueStreamResult{
		Record: record,
		Stream: stream,
		OnComplete: func(content string) (*ContinueResult, error) {
			content = strings.TrimSpace(content)
			if content == "" {
				err := fmt.Errorf("llm response content must not be empty")
				_ = u.failGeneration(ctx, record, "", startedAt, err)
				return nil, err
			}

			updated := *chapterEntity
			updated.Status = chapterdomain.StatusDraft
			updated.Content = content
			updated.CurrentDraftID = recordID
			updated.CurrentDraftConfirmedAt = nil
			updated.CurrentDraftConfirmedBy = ""
			updated.UpdatedAt = time.Now().UTC()
			if err := updated.Validate(); err != nil {
				_ = u.failGeneration(ctx, record, content, startedAt, appservice.WrapInvalidInput(err))
				return nil, appservice.WrapInvalidInput(err)
			}
			updatedOK, err := u.chapters.UpdateIfUnchanged(ctx, &updated, expectedUpdatedAt)
			if err != nil {
				translatedErr := appservice.TranslateStorageError(err)
				_ = u.failGeneration(ctx, record, content, startedAt, translatedErr)
				return nil, translatedErr
			}
			if !updatedOK {
				conflictErr := appservice.WrapConflict(fmt.Errorf("chapter was modified during continuation; please retry"))
				_ = u.failGeneration(ctx, record, content, startedAt, conflictErr)
				return nil, conflictErr
			}
			if err := u.succeedGeneration(ctx, record, content, startedAt); err != nil {
				return nil, err
			}
			return &ContinueResult{Chapter: &updated, GenerationRecord: record}, nil
		},
		OnError: func(err error) {
			_ = u.failGeneration(ctx, record, "", startedAt, err)
		},
	}, nil
}

func (u *useCase) RewriteStream(ctx context.Context, params RewriteParams) (*RewriteStreamResult, error) {
	startedAt := time.Now().UTC()
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
	expectedUpdatedAt := chapterEntity.UpdatedAt
	if !strings.Contains(chapterEntity.Content, trimmedTargetText) {
		return nil, appservice.WrapInvalidInput(fmt.Errorf("target_text must exactly match existing chapter content"))
	}

	projectEntity, promptContext, err := u.loadProjectAndPromptContext(ctx, chapterEntity.ProjectID)
	if err != nil {
		return nil, err
	}

	systemPrompt, userPrompt, err := u.renderPrompt(ctx, chapterEntity.ProjectID, generationdomain.KindChapterRewrite, buildRewritePromptData(projectEntity, promptContext, chapterEntity, params))
	if err != nil {
		return nil, err
	}

	recordID := uuid.NewString()
	record := newGenerationRecord(recordID, chapterEntity.ProjectID, chapterEntity.ID, generationdomain.KindChapterRewrite, buildPromptSnapshot(systemPrompt, userPrompt), startedAt)
	if err := u.generationRecords.Create(ctx, record); err != nil {
		return nil, appservice.TranslateStorageError(err)
	}

	stream, err := u.streamContent(ctx, systemPrompt, userPrompt)
	if err != nil {
		_ = u.failGeneration(ctx, record, "", startedAt, err)
		return nil, err
	}

	return &RewriteStreamResult{
		Record: record,
		Stream: stream,
		OnComplete: func(content string) (*RewriteResult, error) {
			content = strings.TrimSpace(content)
			if content == "" {
				err := fmt.Errorf("llm response content must not be empty")
				_ = u.failGeneration(ctx, record, "", startedAt, err)
				return nil, err
			}

			updated := *chapterEntity
			updated.Status = chapterdomain.StatusDraft
			updated.Content = content
			updated.CurrentDraftID = recordID
			updated.CurrentDraftConfirmedAt = nil
			updated.CurrentDraftConfirmedBy = ""
			updated.UpdatedAt = time.Now().UTC()
			if err := updated.Validate(); err != nil {
				_ = u.failGeneration(ctx, record, content, startedAt, appservice.WrapInvalidInput(err))
				return nil, appservice.WrapInvalidInput(err)
			}
			updatedOK, err := u.chapters.UpdateIfUnchanged(ctx, &updated, expectedUpdatedAt)
			if err != nil {
				translatedErr := appservice.TranslateStorageError(err)
				_ = u.failGeneration(ctx, record, content, startedAt, translatedErr)
				return nil, translatedErr
			}
			if !updatedOK {
				conflictErr := appservice.WrapConflict(fmt.Errorf("chapter was modified during rewrite; please retry"))
				_ = u.failGeneration(ctx, record, content, startedAt, conflictErr)
				return nil, conflictErr
			}
			if err := u.succeedGeneration(ctx, record, content, startedAt); err != nil {
				return nil, err
			}
			return &RewriteResult{Chapter: &updated, GenerationRecord: record}, nil
		},
		OnError: func(err error) {
			_ = u.failGeneration(ctx, record, "", startedAt, err)
		},
	}, nil
}
