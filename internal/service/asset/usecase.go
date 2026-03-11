package asset

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	assetdomain "novelforge/backend/internal/domain/asset"
	generationdomain "novelforge/backend/internal/domain/generation"
	metricdomain "novelforge/backend/internal/domain/metric"
	projectdomain "novelforge/backend/internal/domain/project"
	"novelforge/backend/internal/infra/llm"
	"novelforge/backend/internal/infra/llm/prompts"
	appservice "novelforge/backend/internal/service"
	metricservice "novelforge/backend/internal/service/metric"

	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

type generatedAssetPayload struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type useCase struct {
	assets            assetdomain.AssetRepository
	projects          projectdomain.ProjectRepository
	generationRecords generationdomain.GenerationRecordRepository
	llmClient         llm.Client
	promptStore       *prompts.Store
	metrics           metricservice.UseCase
}

// NewUseCase 创建资产(asset)用例实现。
func NewUseCase(deps Dependencies) UseCase {
	return &useCase{
		assets:            deps.Assets,
		projects:          deps.Projects,
		generationRecords: deps.GenerationRecords,
		llmClient:         deps.LLMClient,
		promptStore:       deps.PromptStore,
		metrics:           deps.Metrics,
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

func (u *useCase) Generate(ctx context.Context, params GenerateParams) (*GenerateResult, error) {
	startedAt := time.Now().UTC()
	params.ProjectID = strings.TrimSpace(params.ProjectID)
	params.Type = strings.TrimSpace(params.Type)
	params.Instruction = strings.TrimSpace(params.Instruction)

	if err := validateAssetProjectID(params.ProjectID); err != nil {
		u.trackOperationFailed(ctx, params.ProjectID, params.Type, "generate", generationdomain.KindAssetGeneration, startedAt, 0, err)
		return nil, err
	}
	if !assetdomain.IsValidType(params.Type) {
		err := appservice.WrapInvalidInput(fmt.Errorf("type must be one of worldbuilding, character, outline"))
		u.trackOperationFailed(ctx, params.ProjectID, params.Type, "generate", generationdomain.KindAssetGeneration, startedAt, 0, err)
		return nil, err
	}
	if params.Instruction == "" {
		err := appservice.WrapInvalidInput(fmt.Errorf("instruction must not be empty"))
		u.trackOperationFailed(ctx, params.ProjectID, params.Type, "generate", generationdomain.KindAssetGeneration, startedAt, 0, err)
		return nil, err
	}

	projectEntity, err := u.projects.GetByID(ctx, params.ProjectID)
	if err != nil {
		translatedErr := appservice.TranslateStorageError(err)
		u.trackOperationFailed(ctx, params.ProjectID, params.Type, "generate", generationdomain.KindAssetGeneration, startedAt, 0, translatedErr)
		return nil, translatedErr
	}

	systemPrompt, userPrompt, err := u.renderPrompt(generationdomain.KindAssetGeneration, map[string]string{
		"ProjectTitle":   projectEntity.Title,
		"ProjectSummary": projectEntity.Summary,
		"AssetType":      params.Type,
		"Instruction":    params.Instruction,
	})
	if err != nil {
		u.trackOperationFailed(ctx, params.ProjectID, params.Type, "generate", generationdomain.KindAssetGeneration, startedAt, 0, err)
		return nil, err
	}

	recordID := uuid.NewString()
	record := newGenerationRecord(recordID, params.ProjectID, generationdomain.KindAssetGeneration, buildPromptSnapshot(systemPrompt, userPrompt), startedAt)
	if err := u.generationRecords.Create(ctx, record); err != nil {
		translatedErr := appservice.TranslateStorageError(err)
		u.trackOperationFailed(ctx, params.ProjectID, params.Type, "generate", generationdomain.KindAssetGeneration, startedAt, 0, translatedErr)
		return nil, translatedErr
	}

	rawOutput, err := u.generateAssetContent(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, u.failGeneration(ctx, record, "", startedAt, params.Type, err)
	}

	// 使用严格 JSON 协议解析模型输出，避免非结构化文本污染资产数据。
	parsed, parseErr := parseGeneratedAsset(rawOutput)
	if parseErr != nil {
		return nil, u.failGeneration(ctx, record, rawOutput, startedAt, params.Type, appservice.WrapInvalidInput(parseErr))
	}

	now := time.Now().UTC()
	entity := &assetdomain.Asset{
		ID:        uuid.NewString(),
		ProjectID: params.ProjectID,
		Type:      params.Type,
		Title:     strings.TrimSpace(parsed.Title),
		Content:   strings.TrimSpace(parsed.Content),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := entity.Validate(); err != nil {
		return nil, u.failGeneration(ctx, record, rawOutput, startedAt, params.Type, appservice.WrapInvalidInput(err))
	}
	if err := u.assets.Create(ctx, entity); err != nil {
		return nil, u.failGeneration(ctx, record, rawOutput, startedAt, params.Type, appservice.TranslateStorageError(err))
	}
	// 成功时 output_ref 记录落库资产 ID，便于后续链路追踪。
	if err := u.succeedGeneration(ctx, record, entity.ID, startedAt, params.Type); err != nil {
		return nil, err
	}

	return &GenerateResult{
		Asset:            entity,
		GenerationRecord: record,
	}, nil
}

func (u *useCase) ensureProjectExists(ctx context.Context, projectID string) error {
	if _, err := u.projects.GetByID(ctx, projectID); err != nil {
		return appservice.TranslateStorageError(err)
	}
	return nil
}

func (u *useCase) renderPrompt(kind string, promptData map[string]string) (string, string, error) {
	if u.promptStore == nil {
		return "", "", fmt.Errorf("prompt store is not configured")
	}
	promptCapability, ok := appservice.PromptCapabilityForGenerationKind(kind)
	if !ok {
		return "", "", fmt.Errorf("unsupported generation kind %q", kind)
	}
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

func (u *useCase) generateAssetContent(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if u.llmClient == nil || u.llmClient.ChatModel() == nil {
		return "", fmt.Errorf("llm client is not configured")
	}

	response, err := u.llmClient.ChatModel().Generate(ctx, []*schema.Message{
		{Role: schema.System, Content: systemPrompt},
		{Role: schema.User, Content: userPrompt},
	})
	if err != nil {
		return "", fmt.Errorf("generate asset content: %w", err)
	}
	if response == nil || strings.TrimSpace(response.Content) == "" {
		return "", fmt.Errorf("llm response content must not be empty")
	}
	return strings.TrimSpace(response.Content), nil
}

func parseGeneratedAsset(raw string) (*generatedAssetPayload, error) {
	decoder := json.NewDecoder(strings.NewReader(strings.TrimSpace(raw)))
	decoder.DisallowUnknownFields()

	var payload generatedAssetPayload
	if err := decoder.Decode(&payload); err != nil {
		return nil, fmt.Errorf("invalid llm json: %w", err)
	}
	if err := ensureNoTrailingJSON(decoder); err != nil {
		return nil, err
	}
	if strings.TrimSpace(payload.Title) == "" {
		return nil, fmt.Errorf("title must not be empty")
	}
	if strings.TrimSpace(payload.Content) == "" {
		return nil, fmt.Errorf("content must not be empty")
	}
	return &payload, nil
}

func ensureNoTrailingJSON(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != nil {
		if err == io.EOF {
			return nil
		}
		return fmt.Errorf("invalid llm json: %w", err)
	}
	return fmt.Errorf("invalid llm json: trailing data is not allowed")
}

func buildPromptSnapshot(systemPrompt, userPrompt string) string {
	return fmt.Sprintf("system:\n%s\n\nuser:\n%s", strings.TrimSpace(systemPrompt), strings.TrimSpace(userPrompt))
}

func (u *useCase) succeedGeneration(ctx context.Context, record *generationdomain.GenerationRecord, outputRef string, startedAt time.Time, assetType string) error {
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
		u.trackOperationFailed(ctx, record.ProjectID, assetType, "generate", record.Kind, startedAt, 0, translatedErr)
		return translatedErr
	}
	record.Status = params.Status
	record.OutputRef = params.OutputRef
	record.TokenUsage = params.TokenUsage
	record.DurationMillis = params.DurationMillis
	record.ErrorMessage = params.ErrorMessage
	record.UpdatedAt = params.UpdatedAt
	u.trackOperationSucceeded(ctx, record.ProjectID, assetType, "generate", record.Kind, startedAt, record.TokenUsage)
	return nil
}

func (u *useCase) failGeneration(ctx context.Context, record *generationdomain.GenerationRecord, outputRef string, startedAt time.Time, assetType string, cause error) error {
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
		u.trackOperationFailed(ctx, record.ProjectID, assetType, "generate", record.Kind, startedAt, 0, wrappedErr)
		return wrappedErr
	}
	record.Status = params.Status
	record.OutputRef = params.OutputRef
	record.TokenUsage = params.TokenUsage
	record.DurationMillis = params.DurationMillis
	record.ErrorMessage = params.ErrorMessage
	record.UpdatedAt = params.UpdatedAt
	u.trackOperationFailed(ctx, record.ProjectID, assetType, "generate", record.Kind, startedAt, record.TokenUsage, cause)
	return cause
}

func (u *useCase) trackOperationSucceeded(ctx context.Context, projectID, assetType, action, generationKind string, startedAt time.Time, tokenUsage int) {
	if strings.TrimSpace(action) == "" {
		return
	}
	u.appendMetricEvent(ctx, metricdomain.EventOperationCompleted, projectID, assetType, action, generationKind, tokenUsage, startedAt, nil)
}

func (u *useCase) trackOperationFailed(ctx context.Context, projectID, assetType, action, generationKind string, startedAt time.Time, tokenUsage int, cause error) {
	if strings.TrimSpace(action) == "" {
		return
	}
	u.appendMetricEvent(ctx, metricdomain.EventOperationFailed, projectID, assetType, action, generationKind, tokenUsage, startedAt, cause)
}

func (u *useCase) appendMetricEvent(ctx context.Context, eventName, projectID, assetType, action, generationKind string, tokenUsage int, startedAt time.Time, cause error) {
	if u.metrics == nil {
		return
	}

	projectID = strings.TrimSpace(projectID)
	if _, err := uuid.Parse(projectID); err != nil {
		// 无法确定项目归属时跳过落库，避免污染事件表。
		log.Printf("metric append skipped event_name=%s action=%s reason=invalid_project_id", eventName, action)
		return
	}

	labels := map[string]string{
		"domain": "asset",
		"action": action,
	}
	assetType = strings.TrimSpace(assetType)
	if assetType != "" {
		labels["asset_type"] = assetType
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
		Labels:    labels,
		Stats: map[string]float64{
			"duration_ms": float64(durationMillis(startedAt, time.Now().UTC())),
			"token_usage": float64(tokenUsage),
		},
		OccurredAt: time.Now().UTC(),
	}
	if err := u.metrics.Append(ctx, event); err != nil {
		// 埋点失败不影响主业务流程，仅记录 warning 以便排查。
		log.Printf("metric append failed event_name=%s action=%s project_id=%s err=%v", eventName, action, projectID, err)
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

func newGenerationRecord(id, projectID, kind, inputSnapshot string, now time.Time) *generationdomain.GenerationRecord {
	return &generationdomain.GenerationRecord{
		ID:               id,
		ProjectID:        projectID,
		ChapterID:        "",
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

func validateAssetID(id string) error {
	if _, err := uuid.Parse(strings.TrimSpace(id)); err != nil {
		return appservice.WrapInvalidInput(fmt.Errorf("id must be a valid UUID"))
	}
	return nil
}

func validateAssetProjectID(projectID string) error {
	if _, err := uuid.Parse(strings.TrimSpace(projectID)); err != nil {
		return appservice.WrapInvalidInput(fmt.Errorf("project_id must be a valid UUID"))
	}
	return nil
}
