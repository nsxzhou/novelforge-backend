package conversation

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
	conversationdomain "novelforge/backend/internal/domain/conversation"
	metricdomain "novelforge/backend/internal/domain/metric"
	projectdomain "novelforge/backend/internal/domain/project"
	promptdomain "novelforge/backend/internal/domain/prompt"
	"novelforge/backend/internal/infra/llm"
	"novelforge/backend/internal/infra/llm/prompts"
	appservice "novelforge/backend/internal/service"
	metricservice "novelforge/backend/internal/service/metric"
	"novelforge/backend/pkg/config"

	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

type useCase struct {
	conversations   conversationdomain.ConversationRepository
	projects        projectdomain.ProjectRepository
	assets          assetdomain.AssetRepository
	llmClient       llm.Client
	promptStore     *prompts.Store
	promptOverrides promptdomain.OverrideRepository
	metrics         metricservice.UseCase
	txRunner        TxRunner
}

// NewUseCase 创建对话(conversation)细化用例实现。
func NewUseCase(deps Dependencies) UseCase {
	return &useCase{
		conversations:   deps.Conversations,
		projects:        deps.Projects,
		assets:          deps.Assets,
		llmClient:       deps.LLMClient,
		promptStore:     deps.PromptStore,
		promptOverrides: deps.PromptOverrides,
		metrics:         deps.Metrics,
		txRunner:        deps.TxRunner,
	}
}

func (u *useCase) Start(ctx context.Context, params StartParams) (*conversationdomain.Conversation, error) {
	startedAt := time.Now().UTC()
	params.ProjectID = strings.TrimSpace(params.ProjectID)
	params.TargetType = strings.TrimSpace(params.TargetType)
	params.TargetID = strings.TrimSpace(params.TargetID)
	params.Message = strings.TrimSpace(params.Message)

	if err := validateProjectID(params.ProjectID); err != nil {
		u.trackOperationFailed(ctx, params.ProjectID, params.TargetType, "start", startedAt, 0, err)
		return nil, err
	}
	if err := validateTarget(params.TargetType, params.TargetID); err != nil {
		u.trackOperationFailed(ctx, params.ProjectID, params.TargetType, "start", startedAt, 0, err)
		return nil, err
	}
	if params.Message == "" {
		err := appservice.WrapInvalidInput(fmt.Errorf("message must not be empty"))
		u.trackOperationFailed(ctx, params.ProjectID, params.TargetType, "start", startedAt, 0, err)
		return nil, err
	}

	target, err := u.loadTarget(ctx, params.ProjectID, params.TargetType, params.TargetID)
	if err != nil {
		u.trackOperationFailed(ctx, params.ProjectID, params.TargetType, "start", startedAt, 0, err)
		return nil, err
	}

	now := time.Now().UTC()
	conversation := &conversationdomain.Conversation{
		ID:         uuid.NewString(),
		ProjectID:  params.ProjectID,
		TargetType: params.TargetType,
		TargetID:   params.TargetID,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	userMessage := newConversationMessage(conversationdomain.MessageRoleUser, params.Message, now)
	if err := conversation.AppendMessage(userMessage); err != nil {
		wrappedErr := appservice.WrapInvalidInput(err)
		u.trackOperationFailed(ctx, params.ProjectID, params.TargetType, "start", startedAt, 0, wrappedErr)
		return nil, wrappedErr
	}

	suggestion, assistantMessage, err := u.generateSuggestion(ctx, conversation, target, params.Message)
	if err != nil {
		u.trackOperationFailed(ctx, params.ProjectID, params.TargetType, "start", startedAt, 0, err)
		return nil, err
	}
	if err := conversation.AppendMessage(assistantMessage); err != nil {
		wrappedErr := appservice.WrapInvalidInput(err)
		u.trackOperationFailed(ctx, params.ProjectID, params.TargetType, "start", startedAt, 0, wrappedErr)
		return nil, wrappedErr
	}
	if err := conversation.ReplacePendingSuggestion(*suggestion, assistantMessage.CreatedAt); err != nil {
		wrappedErr := appservice.WrapInvalidInput(err)
		u.trackOperationFailed(ctx, params.ProjectID, params.TargetType, "start", startedAt, 0, wrappedErr)
		return nil, wrappedErr
	}

	if err := u.conversations.Create(ctx, conversation); err != nil {
		translatedErr := appservice.TranslateStorageError(err)
		u.trackOperationFailed(ctx, params.ProjectID, params.TargetType, "start", startedAt, 0, translatedErr)
		return nil, translatedErr
	}
	u.trackOperationSucceeded(ctx, params.ProjectID, params.TargetType, "start", startedAt, 0)
	return conversation, nil
}

func (u *useCase) Reply(ctx context.Context, params ReplyParams) (*conversationdomain.Conversation, error) {
	startedAt := time.Now().UTC()
	params.ConversationID = strings.TrimSpace(params.ConversationID)
	params.Message = strings.TrimSpace(params.Message)

	if err := validateConversationID(params.ConversationID); err != nil {
		u.trackOperationFailed(ctx, "", "", "reply", startedAt, 0, err)
		return nil, err
	}
	if params.Message == "" {
		err := appservice.WrapInvalidInput(fmt.Errorf("message must not be empty"))
		u.trackOperationFailed(ctx, "", "", "reply", startedAt, 0, err)
		return nil, err
	}

	conversation, err := u.conversations.GetByID(ctx, params.ConversationID)
	if err != nil {
		translatedErr := appservice.TranslateStorageError(err)
		u.trackOperationFailed(ctx, "", "", "reply", startedAt, 0, translatedErr)
		return nil, translatedErr
	}

	target, err := u.loadTarget(ctx, conversation.ProjectID, conversation.TargetType, conversation.TargetID)
	if err != nil {
		u.trackOperationFailed(ctx, conversation.ProjectID, conversation.TargetType, "reply", startedAt, 0, err)
		return nil, err
	}

	now := time.Now().UTC()
	userMessage := newConversationMessage(conversationdomain.MessageRoleUser, params.Message, now)
	if err := conversation.AppendMessage(userMessage); err != nil {
		wrappedErr := appservice.WrapInvalidInput(err)
		u.trackOperationFailed(ctx, conversation.ProjectID, conversation.TargetType, "reply", startedAt, 0, wrappedErr)
		return nil, wrappedErr
	}

	suggestion, assistantMessage, err := u.generateSuggestion(ctx, conversation, target, params.Message)
	if err != nil {
		u.trackOperationFailed(ctx, conversation.ProjectID, conversation.TargetType, "reply", startedAt, 0, err)
		return nil, err
	}
	if err := conversation.AppendMessage(assistantMessage); err != nil {
		wrappedErr := appservice.WrapInvalidInput(err)
		u.trackOperationFailed(ctx, conversation.ProjectID, conversation.TargetType, "reply", startedAt, 0, wrappedErr)
		return nil, wrappedErr
	}
	if err := conversation.ReplacePendingSuggestion(*suggestion, assistantMessage.CreatedAt); err != nil {
		wrappedErr := appservice.WrapInvalidInput(err)
		u.trackOperationFailed(ctx, conversation.ProjectID, conversation.TargetType, "reply", startedAt, 0, wrappedErr)
		return nil, wrappedErr
	}

	if err := u.conversations.Update(ctx, conversation); err != nil {
		translatedErr := appservice.TranslateStorageError(err)
		u.trackOperationFailed(ctx, conversation.ProjectID, conversation.TargetType, "reply", startedAt, 0, translatedErr)
		return nil, translatedErr
	}
	u.trackOperationSucceeded(ctx, conversation.ProjectID, conversation.TargetType, "reply", startedAt, 0)
	return conversation, nil
}

func (u *useCase) Confirm(ctx context.Context, conversationID string) (*ConfirmResult, error) {
	startedAt := time.Now().UTC()
	conversationID = strings.TrimSpace(conversationID)
	if err := validateConversationID(conversationID); err != nil {
		u.trackOperationFailed(ctx, "", "", "confirm", startedAt, 0, err)
		return nil, err
	}

	result, err := u.confirmInTx(ctx, conversationID)
	if err != nil {
		projectID := ""
		targetType := ""
		if result != nil && result.Conversation != nil {
			projectID = result.Conversation.ProjectID
			targetType = result.Conversation.TargetType
		}
		u.trackOperationFailed(ctx, projectID, targetType, "confirm", startedAt, 0, err)
		return nil, err
	}
	u.trackOperationSucceeded(ctx, result.Conversation.ProjectID, result.Conversation.TargetType, "confirm", startedAt, 0)
	return result, nil
}

func (u *useCase) confirmInTx(ctx context.Context, conversationID string) (*ConfirmResult, error) {
	result := &ConfirmResult{}
	run := u.txRunner
	if run == nil {
		run = noTxRunner{}
	}

	if err := run.InTx(ctx, func(txCtx context.Context) error {
		conversation, err := u.conversations.GetByID(txCtx, conversationID)
		if err != nil {
			return appservice.TranslateStorageError(err)
		}
		if conversation.PendingSuggestion == nil {
			return appservice.WrapInvalidInput(fmt.Errorf("pending_suggestion must not be empty"))
		}
		result.Conversation = conversation

		expectedConversationUpdatedAt := nonZeroUpdatedAt(conversation.CreatedAt, conversation.UpdatedAt)
		conversationUpdatedAt := nonZeroUpdatedAt(expectedConversationUpdatedAt, time.Now().UTC())

		switch conversation.TargetType {
		case conversationdomain.TargetTypeProject:
			projectEntity, loadErr := u.projects.GetByID(txCtx, conversation.TargetID)
			if loadErr != nil {
				return appservice.TranslateStorageError(loadErr)
			}
			expectedProjectUpdatedAt := nonZeroUpdatedAt(projectEntity.CreatedAt, projectEntity.UpdatedAt)
			entityUpdatedAt := nonZeroUpdatedAt(expectedProjectUpdatedAt, conversationUpdatedAt)
			updatedProject := *projectEntity
			updatedProject.Title = strings.TrimSpace(conversation.PendingSuggestion.Title)
			updatedProject.Summary = strings.TrimSpace(conversation.PendingSuggestion.Summary)
			updatedProject.UpdatedAt = entityUpdatedAt
			if err := updatedProject.Validate(); err != nil {
				return appservice.WrapInvalidInput(err)
			}
			updatedOK, updateErr := u.projects.UpdateIfUnchanged(txCtx, &updatedProject, expectedProjectUpdatedAt)
			if updateErr != nil {
				return appservice.TranslateStorageError(updateErr)
			}
			if !updatedOK {
				return appservice.WrapConflict(fmt.Errorf("project was modified during confirmation; please retry"))
			}
			result.Project = &updatedProject
		case conversationdomain.TargetTypeAsset:
			assetEntity, loadErr := u.assets.GetByID(txCtx, conversation.TargetID)
			if loadErr != nil {
				return appservice.TranslateStorageError(loadErr)
			}
			if assetEntity.ProjectID != conversation.ProjectID {
				return appservice.WrapInvalidInput(fmt.Errorf("asset does not belong to project"))
			}
			expectedAssetUpdatedAt := nonZeroUpdatedAt(assetEntity.CreatedAt, assetEntity.UpdatedAt)
			entityUpdatedAt := nonZeroUpdatedAt(expectedAssetUpdatedAt, conversationUpdatedAt)
			updatedAsset := *assetEntity
			updatedAsset.Title = strings.TrimSpace(conversation.PendingSuggestion.Title)
			updatedAsset.Content = strings.TrimSpace(conversation.PendingSuggestion.Content)
			updatedAsset.UpdatedAt = entityUpdatedAt
			if err := updatedAsset.Validate(); err != nil {
				return appservice.WrapInvalidInput(err)
			}
			updatedOK, updateErr := u.assets.UpdateIfUnchanged(txCtx, &updatedAsset, expectedAssetUpdatedAt)
			if updateErr != nil {
				return appservice.TranslateStorageError(updateErr)
			}
			if !updatedOK {
				return appservice.WrapConflict(fmt.Errorf("asset was modified during confirmation; please retry"))
			}
			result.Asset = &updatedAsset
		default:
			return appservice.WrapInvalidInput(fmt.Errorf("target_type must be one of project, asset"))
		}

		updatedConversation := *conversation
		updatedConversation.ClearPendingSuggestion(conversationUpdatedAt)
		systemMessage := newConversationMessage(conversationdomain.MessageRoleSystem, confirmedMessageContent(conversation.TargetType), conversationUpdatedAt)
		if err := updatedConversation.AppendMessage(systemMessage); err != nil {
			return appservice.WrapInvalidInput(err)
		}
		updatedOK, updateErr := u.conversations.UpdateIfUnchanged(txCtx, &updatedConversation, expectedConversationUpdatedAt)
		if updateErr != nil {
			return appservice.TranslateStorageError(updateErr)
		}
		if !updatedOK {
			return appservice.WrapConflict(fmt.Errorf("conversation was modified during confirmation; please retry"))
		}
		result.Conversation = &updatedConversation
		return nil
	}); err != nil {
		return result, err
	}

	return result, nil
}

type noTxRunner struct{}

func (noTxRunner) InTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func (u *useCase) GetByID(ctx context.Context, id string) (*conversationdomain.Conversation, error) {
	id = strings.TrimSpace(id)
	if err := validateConversationID(id); err != nil {
		return nil, err
	}
	conversation, err := u.conversations.GetByID(ctx, id)
	if err != nil {
		return nil, appservice.TranslateStorageError(err)
	}
	return conversation, nil
}

func (u *useCase) List(ctx context.Context, params ListParams) ([]*conversationdomain.Conversation, error) {
	params.ProjectID = strings.TrimSpace(params.ProjectID)
	params.TargetType = strings.TrimSpace(params.TargetType)
	params.TargetID = strings.TrimSpace(params.TargetID)

	if err := validateProjectID(params.ProjectID); err != nil {
		return nil, err
	}
	if _, err := u.projects.GetByID(ctx, params.ProjectID); err != nil {
		return nil, appservice.TranslateStorageError(err)
	}

	if params.TargetType == "" && params.TargetID == "" {
		items, err := u.conversations.ListByProject(ctx, conversationdomain.ListByProjectParams{
			ProjectID: params.ProjectID,
			Limit:     params.Limit,
			Offset:    params.Offset,
		})
		if err != nil {
			return nil, appservice.TranslateStorageError(err)
		}
		return items, nil
	}
	if params.TargetType == "" || params.TargetID == "" {
		return nil, appservice.WrapInvalidInput(fmt.Errorf("target_type and target_id must be provided together"))
	}
	if err := validateTarget(params.TargetType, params.TargetID); err != nil {
		return nil, err
	}
	if _, err := u.loadTarget(ctx, params.ProjectID, params.TargetType, params.TargetID); err != nil {
		return nil, err
	}

	items, err := u.conversations.ListByTarget(ctx, conversationdomain.ListByTargetParams{
		ProjectID:  params.ProjectID,
		TargetType: params.TargetType,
		TargetID:   params.TargetID,
		Limit:      params.Limit,
		Offset:     params.Offset,
	})
	if err != nil {
		return nil, appservice.TranslateStorageError(err)
	}
	return items, nil
}

func (u *useCase) generateSuggestion(ctx context.Context, conversation *conversationdomain.Conversation, target any, latestUserMessage string) (*conversationdomain.PendingSuggestion, conversationdomain.Message, error) {
	promptCapability, ok := appservice.PromptCapabilityForConversationTarget(conversation.TargetType)
	if !ok {
		return nil, conversationdomain.Message{}, appservice.WrapInvalidInput(fmt.Errorf("target_type must be one of project, asset"))
	}

	promptData := buildPromptData(conversation, target, latestUserMessage)

	systemPrompt, userPrompt, err := u.renderConversationPrompt(ctx, conversation.ProjectID, promptCapability, promptData)
	if err != nil {
		return nil, conversationdomain.Message{}, err
	}

	if u.llmClient == nil || u.llmClient.ChatModel() == nil {
		return nil, conversationdomain.Message{}, fmt.Errorf("llm client is not configured")
	}

	response, err := u.llmClient.ChatModel().Generate(ctx, []*schema.Message{
		{Role: schema.System, Content: systemPrompt},
		{Role: schema.User, Content: userPrompt},
	})
	if err != nil {
		return nil, conversationdomain.Message{}, fmt.Errorf("generate refinement suggestion: %w", err)
	}
	if response == nil || strings.TrimSpace(response.Content) == "" {
		return nil, conversationdomain.Message{}, fmt.Errorf("llm response content must not be empty")
	}

	suggestion, err := parseSuggestion(response.Content, conversation.TargetType)
	if err != nil {
		return nil, conversationdomain.Message{}, appservice.WrapInvalidInput(err)
	}
	if err := validateSuggestionAgainstTarget(suggestion, conversation.TargetType, target); err != nil {
		return nil, conversationdomain.Message{}, err
	}

	assistantMessage := newConversationMessage(conversationdomain.MessageRoleAssistant, response.Content, time.Now().UTC())
	return suggestion, assistantMessage, nil
}

func (u *useCase) renderConversationPrompt(ctx context.Context, projectID string, promptCapability config.PromptCapability, promptData map[string]string) (string, string, error) {
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

func (u *useCase) loadTarget(ctx context.Context, projectID, targetType, targetID string) (any, error) {
	projectEntity, err := u.projects.GetByID(ctx, projectID)
	if err != nil {
		return nil, appservice.TranslateStorageError(err)
	}

	switch targetType {
	case conversationdomain.TargetTypeProject:
		if projectEntity.ID != targetID {
			return nil, appservice.WrapInvalidInput(fmt.Errorf("project target_id must match project_id"))
		}
		return projectEntity, nil
	case conversationdomain.TargetTypeAsset:
		assetEntity, err := u.assets.GetByID(ctx, targetID)
		if err != nil {
			return nil, appservice.TranslateStorageError(err)
		}
		if assetEntity.ProjectID != projectID {
			return nil, appservice.WrapInvalidInput(fmt.Errorf("asset does not belong to project"))
		}
		return assetEntity, nil
	default:
		return nil, appservice.WrapInvalidInput(fmt.Errorf("target_type must be one of project, asset"))
	}
}

func parseSuggestion(raw, targetType string) (*conversationdomain.PendingSuggestion, error) {
	decoder := json.NewDecoder(strings.NewReader(strings.TrimSpace(raw)))
	decoder.DisallowUnknownFields()

	suggestion := &conversationdomain.PendingSuggestion{}
	switch targetType {
	case conversationdomain.TargetTypeProject:
		var payload struct {
			Title   string `json:"title"`
			Summary string `json:"summary"`
		}
		if err := decoder.Decode(&payload); err != nil {
			return nil, fmt.Errorf("invalid llm json: %w", err)
		}
		suggestion.Title = payload.Title
		suggestion.Summary = payload.Summary
	case conversationdomain.TargetTypeAsset:
		var payload struct {
			Title   string `json:"title"`
			Content string `json:"content"`
		}
		if err := decoder.Decode(&payload); err != nil {
			return nil, fmt.Errorf("invalid llm json: %w", err)
		}
		suggestion.Title = payload.Title
		suggestion.Content = payload.Content
	default:
		return nil, fmt.Errorf("target_type must be one of project, asset")
	}

	if err := ensureNoTrailingJSON(decoder); err != nil {
		return nil, err
	}
	if err := suggestion.Validate(targetType); err != nil {
		return nil, err
	}
	return suggestion, nil
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

func validateSuggestionAgainstTarget(suggestion *conversationdomain.PendingSuggestion, targetType string, target any) error {
	switch targetType {
	case conversationdomain.TargetTypeProject:
		entity, ok := target.(*projectdomain.Project)
		if !ok || entity == nil {
			return fmt.Errorf("project target is not available")
		}
		candidate := &projectdomain.Project{
			ID:        entity.ID,
			Title:     strings.TrimSpace(suggestion.Title),
			Summary:   strings.TrimSpace(suggestion.Summary),
			Status:    entity.Status,
			CreatedAt: entity.CreatedAt,
			UpdatedAt: nonZeroUpdatedAt(entity.UpdatedAt, time.Now().UTC()),
		}
		if err := candidate.Validate(); err != nil {
			return appservice.WrapInvalidInput(err)
		}
	case conversationdomain.TargetTypeAsset:
		entity, ok := target.(*assetdomain.Asset)
		if !ok || entity == nil {
			return fmt.Errorf("asset target is not available")
		}
		candidate := &assetdomain.Asset{
			ID:        entity.ID,
			ProjectID: entity.ProjectID,
			Type:      entity.Type,
			Title:     strings.TrimSpace(suggestion.Title),
			Content:   strings.TrimSpace(suggestion.Content),
			CreatedAt: entity.CreatedAt,
			UpdatedAt: nonZeroUpdatedAt(entity.UpdatedAt, time.Now().UTC()),
		}
		if err := candidate.Validate(); err != nil {
			return appservice.WrapInvalidInput(err)
		}
	default:
		return appservice.WrapInvalidInput(fmt.Errorf("target_type must be one of project, asset"))
	}
	return nil
}

func nonZeroUpdatedAt(createdAt, candidate time.Time) time.Time {
	if candidate.IsZero() || candidate.Before(createdAt) {
		return createdAt
	}
	return candidate
}

func buildPromptData(conversation *conversationdomain.Conversation, target any, latestUserMessage string) map[string]string {
	data := map[string]string{
		"ConversationHistory": formatConversationHistory(conversation.Messages),
		"LatestUserMessage":   strings.TrimSpace(latestUserMessage),
	}

	switch entity := target.(type) {
	case *projectdomain.Project:
		data["ProjectTitle"] = entity.Title
		data["ProjectSummary"] = entity.Summary
	case *assetdomain.Asset:
		data["AssetTitle"] = entity.Title
		data["AssetContent"] = entity.Content
		data["AssetType"] = entity.Type
	}
	return data
}

func formatConversationHistory(messages []conversationdomain.Message) string {
	if len(messages) == 0 {
		return ""
	}
	parts := make([]string, 0, len(messages))
	for _, message := range messages {
		parts = append(parts, fmt.Sprintf("%s: %s", message.Role, strings.TrimSpace(message.Content)))
	}
	return strings.Join(parts, "\n")
}

func newConversationMessage(role, content string, createdAt time.Time) conversationdomain.Message {
	return conversationdomain.Message{
		ID:        uuid.NewString(),
		Role:      role,
		Content:   strings.TrimSpace(content),
		CreatedAt: createdAt,
	}
}

func confirmedMessageContent(targetType string) string {
	switch targetType {
	case conversationdomain.TargetTypeProject:
		return "已确认最新项目建议并写回项目。"
	case conversationdomain.TargetTypeAsset:
		return "已确认最新资产建议并写回资产。"
	default:
		return "已确认最新建议。"
	}
}

func (u *useCase) trackOperationSucceeded(ctx context.Context, projectID, targetType, action string, startedAt time.Time, tokenUsage int) {
	if strings.TrimSpace(action) == "" {
		return
	}
	u.appendMetricEvent(ctx, metricdomain.EventOperationCompleted, projectID, targetType, action, tokenUsage, startedAt, nil)
}

func (u *useCase) trackOperationFailed(ctx context.Context, projectID, targetType, action string, startedAt time.Time, tokenUsage int, cause error) {
	if strings.TrimSpace(action) == "" {
		return
	}
	u.appendMetricEvent(ctx, metricdomain.EventOperationFailed, projectID, targetType, action, tokenUsage, startedAt, cause)
}

func (u *useCase) appendMetricEvent(ctx context.Context, eventName, projectID, targetType, action string, tokenUsage int, startedAt time.Time, cause error) {
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
		"domain": "conversation",
		"action": action,
	}
	targetType = strings.TrimSpace(targetType)
	if targetType != "" {
		labels["target_type"] = targetType
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

func validateProjectID(id string) error {
	if _, err := uuid.Parse(strings.TrimSpace(id)); err != nil {
		return appservice.WrapInvalidInput(fmt.Errorf("project_id must be a valid UUID"))
	}
	return nil
}

func validateConversationID(id string) error {
	if _, err := uuid.Parse(strings.TrimSpace(id)); err != nil {
		return appservice.WrapInvalidInput(fmt.Errorf("conversation_id must be a valid UUID"))
	}
	return nil
}

func validateTarget(targetType, targetID string) error {
	if !conversationdomain.IsValidTargetType(targetType) {
		return appservice.WrapInvalidInput(fmt.Errorf("target_type must be one of project, asset"))
	}
	if _, err := uuid.Parse(strings.TrimSpace(targetID)); err != nil {
		return appservice.WrapInvalidInput(fmt.Errorf("target_id must be a valid UUID"))
	}
	return nil
}

func (u *useCase) streamLLMContent(ctx context.Context, systemPrompt, userPrompt string) (*schema.StreamReader[*schema.Message], error) {
	if u.llmClient == nil || u.llmClient.ChatModel() == nil {
		return nil, fmt.Errorf("llm client is not configured")
	}

	stream, err := u.llmClient.ChatModel().Stream(ctx, []*schema.Message{
		{Role: schema.System, Content: systemPrompt},
		{Role: schema.User, Content: userPrompt},
	})
	if err != nil {
		return nil, fmt.Errorf("stream refinement suggestion: %w", err)
	}
	return stream, nil
}

func (u *useCase) StartStream(ctx context.Context, params StartParams) (*StartStreamResult, error) {
	params.ProjectID = strings.TrimSpace(params.ProjectID)
	params.TargetType = strings.TrimSpace(params.TargetType)
	params.TargetID = strings.TrimSpace(params.TargetID)
	params.Message = strings.TrimSpace(params.Message)

	if err := validateProjectID(params.ProjectID); err != nil {
		return nil, err
	}
	if err := validateTarget(params.TargetType, params.TargetID); err != nil {
		return nil, err
	}
	if params.Message == "" {
		return nil, appservice.WrapInvalidInput(fmt.Errorf("message must not be empty"))
	}

	target, err := u.loadTarget(ctx, params.ProjectID, params.TargetType, params.TargetID)
	if err != nil {
		return nil, err
	}

	promptCapability, ok := appservice.PromptCapabilityForConversationTarget(params.TargetType)
	if !ok {
		return nil, appservice.WrapInvalidInput(fmt.Errorf("target_type must be one of project, asset"))
	}

	now := time.Now().UTC()
	conversation := &conversationdomain.Conversation{
		ID:         uuid.NewString(),
		ProjectID:  params.ProjectID,
		TargetType: params.TargetType,
		TargetID:   params.TargetID,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	userMessage := newConversationMessage(conversationdomain.MessageRoleUser, params.Message, now)
	if err := conversation.AppendMessage(userMessage); err != nil {
		return nil, appservice.WrapInvalidInput(err)
	}

	promptData := buildPromptData(conversation, target, params.Message)
	systemPrompt, userPrompt, err := u.renderConversationPrompt(ctx, params.ProjectID, promptCapability, promptData)
	if err != nil {
		return nil, err
	}

	stream, err := u.streamLLMContent(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	return &StartStreamResult{
		Conversation: conversation,
		Stream:       stream,
		OnComplete: func(content string) (*conversationdomain.Conversation, error) {
			content = strings.TrimSpace(content)
			if content == "" {
				return nil, fmt.Errorf("llm response content must not be empty")
			}

			suggestion, err := parseSuggestion(content, conversation.TargetType)
			if err != nil {
				return nil, appservice.WrapInvalidInput(err)
			}
			if err := validateSuggestionAgainstTarget(suggestion, conversation.TargetType, target); err != nil {
				return nil, err
			}

			assistantMessage := newConversationMessage(conversationdomain.MessageRoleAssistant, content, time.Now().UTC())
			if err := conversation.AppendMessage(assistantMessage); err != nil {
				return nil, appservice.WrapInvalidInput(err)
			}
			if err := conversation.ReplacePendingSuggestion(*suggestion, assistantMessage.CreatedAt); err != nil {
				return nil, appservice.WrapInvalidInput(err)
			}
			if err := u.conversations.Create(ctx, conversation); err != nil {
				return nil, appservice.TranslateStorageError(err)
			}
			return conversation, nil
		},
		OnError: func(err error) {
			// Nothing to clean up for Start since conversation hasn't been persisted yet.
		},
	}, nil
}

func (u *useCase) ReplyStream(ctx context.Context, params ReplyParams) (*ReplyStreamResult, error) {
	params.ConversationID = strings.TrimSpace(params.ConversationID)
	params.Message = strings.TrimSpace(params.Message)

	if err := validateConversationID(params.ConversationID); err != nil {
		return nil, err
	}
	if params.Message == "" {
		return nil, appservice.WrapInvalidInput(fmt.Errorf("message must not be empty"))
	}

	conversation, err := u.conversations.GetByID(ctx, params.ConversationID)
	if err != nil {
		return nil, appservice.TranslateStorageError(err)
	}

	target, err := u.loadTarget(ctx, conversation.ProjectID, conversation.TargetType, conversation.TargetID)
	if err != nil {
		return nil, err
	}

	promptCapability, ok := appservice.PromptCapabilityForConversationTarget(conversation.TargetType)
	if !ok {
		return nil, appservice.WrapInvalidInput(fmt.Errorf("target_type must be one of project, asset"))
	}

	now := time.Now().UTC()
	userMessage := newConversationMessage(conversationdomain.MessageRoleUser, params.Message, now)
	if err := conversation.AppendMessage(userMessage); err != nil {
		return nil, appservice.WrapInvalidInput(err)
	}

	promptData := buildPromptData(conversation, target, params.Message)
	systemPrompt, userPrompt, err := u.renderConversationPrompt(ctx, conversation.ProjectID, promptCapability, promptData)
	if err != nil {
		return nil, err
	}

	stream, err := u.streamLLMContent(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	return &ReplyStreamResult{
		Conversation: conversation,
		Stream:       stream,
		OnComplete: func(content string) (*conversationdomain.Conversation, error) {
			content = strings.TrimSpace(content)
			if content == "" {
				return nil, fmt.Errorf("llm response content must not be empty")
			}

			suggestion, err := parseSuggestion(content, conversation.TargetType)
			if err != nil {
				return nil, appservice.WrapInvalidInput(err)
			}
			if err := validateSuggestionAgainstTarget(suggestion, conversation.TargetType, target); err != nil {
				return nil, err
			}

			assistantMessage := newConversationMessage(conversationdomain.MessageRoleAssistant, content, time.Now().UTC())
			if err := conversation.AppendMessage(assistantMessage); err != nil {
				return nil, appservice.WrapInvalidInput(err)
			}
			if err := conversation.ReplacePendingSuggestion(*suggestion, assistantMessage.CreatedAt); err != nil {
				return nil, appservice.WrapInvalidInput(err)
			}
			if err := u.conversations.Update(ctx, conversation); err != nil {
				return nil, appservice.TranslateStorageError(err)
			}
			return conversation, nil
		},
		OnError: func(err error) {
			// Nothing to clean up for Reply since conversation was already persisted.
		},
	}, nil
}
