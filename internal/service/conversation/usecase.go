package conversation

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	assetdomain "novelforge/backend/internal/domain/asset"
	conversationdomain "novelforge/backend/internal/domain/conversation"
	projectdomain "novelforge/backend/internal/domain/project"
	"novelforge/backend/internal/infra/llm"
	"novelforge/backend/internal/infra/llm/prompts"
	appservice "novelforge/backend/internal/service"

	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

const (
	projectRefinementPromptKind = "project_refinement"
	assetRefinementPromptKind   = "asset_refinement"
)

type useCase struct {
	conversations conversationdomain.ConversationRepository
	projects      projectdomain.ProjectRepository
	assets        assetdomain.AssetRepository
	llmClient     llm.Client
	promptStore   *prompts.Store
}

// NewUseCase 创建对话(conversation)细化用例实现。
func NewUseCase(deps Dependencies) UseCase {
	return &useCase{
		conversations: deps.Conversations,
		projects:      deps.Projects,
		assets:        deps.Assets,
		llmClient:     deps.LLMClient,
		promptStore:   deps.PromptStore,
	}
}

func (u *useCase) Start(ctx context.Context, params StartParams) (*conversationdomain.Conversation, error) {
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

	suggestion, assistantMessage, err := u.generateSuggestion(ctx, conversation, target, params.Message)
	if err != nil {
		return nil, err
	}
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
}

func (u *useCase) Reply(ctx context.Context, params ReplyParams) (*conversationdomain.Conversation, error) {
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

	now := time.Now().UTC()
	userMessage := newConversationMessage(conversationdomain.MessageRoleUser, params.Message, now)
	if err := conversation.AppendMessage(userMessage); err != nil {
		return nil, appservice.WrapInvalidInput(err)
	}

	suggestion, assistantMessage, err := u.generateSuggestion(ctx, conversation, target, params.Message)
	if err != nil {
		return nil, err
	}
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
}

func (u *useCase) Confirm(ctx context.Context, conversationID string) (*ConfirmResult, error) {
	conversationID = strings.TrimSpace(conversationID)
	if err := validateConversationID(conversationID); err != nil {
		return nil, err
	}

	conversation, err := u.conversations.GetByID(ctx, conversationID)
	if err != nil {
		return nil, appservice.TranslateStorageError(err)
	}
	if conversation.PendingSuggestion == nil {
		return nil, appservice.WrapInvalidInput(fmt.Errorf("pending_suggestion must not be empty"))
	}

	result := &ConfirmResult{Conversation: conversation}
	conversationUpdatedAt := nonZeroUpdatedAt(conversation.UpdatedAt, time.Now().UTC())

	switch conversation.TargetType {
	case conversationdomain.TargetTypeProject:
		projectEntity, loadErr := u.projects.GetByID(ctx, conversation.TargetID)
		if loadErr != nil {
			return nil, appservice.TranslateStorageError(loadErr)
		}
		entityUpdatedAt := nonZeroUpdatedAt(projectEntity.UpdatedAt, conversationUpdatedAt)
		projectEntity.Title = strings.TrimSpace(conversation.PendingSuggestion.Title)
		projectEntity.Summary = strings.TrimSpace(conversation.PendingSuggestion.Summary)
		projectEntity.UpdatedAt = entityUpdatedAt
		if err := projectEntity.Validate(); err != nil {
			return nil, appservice.WrapInvalidInput(err)
		}
		if err := u.projects.Update(ctx, projectEntity); err != nil {
			return nil, appservice.TranslateStorageError(err)
		}
		result.Project = projectEntity
	case conversationdomain.TargetTypeAsset:
		assetEntity, loadErr := u.assets.GetByID(ctx, conversation.TargetID)
		if loadErr != nil {
			return nil, appservice.TranslateStorageError(loadErr)
		}
		if assetEntity.ProjectID != conversation.ProjectID {
			return nil, appservice.WrapInvalidInput(fmt.Errorf("asset does not belong to project"))
		}
		entityUpdatedAt := nonZeroUpdatedAt(assetEntity.UpdatedAt, conversationUpdatedAt)
		assetEntity.Title = strings.TrimSpace(conversation.PendingSuggestion.Title)
		assetEntity.Content = strings.TrimSpace(conversation.PendingSuggestion.Content)
		assetEntity.UpdatedAt = entityUpdatedAt
		if err := assetEntity.Validate(); err != nil {
			return nil, appservice.WrapInvalidInput(err)
		}
		if err := u.assets.Update(ctx, assetEntity); err != nil {
			return nil, appservice.TranslateStorageError(err)
		}
		result.Asset = assetEntity
	default:
		return nil, appservice.WrapInvalidInput(fmt.Errorf("target_type must be one of project, asset"))
	}

	conversation.ClearPendingSuggestion(conversationUpdatedAt)
	systemMessage := newConversationMessage(conversationdomain.MessageRoleSystem, confirmedMessageContent(conversation.TargetType), conversationUpdatedAt)
	if err := conversation.AppendMessage(systemMessage); err != nil {
		return nil, appservice.WrapInvalidInput(err)
	}
	if err := u.conversations.Update(ctx, conversation); err != nil {
		return nil, appservice.TranslateStorageError(err)
	}
	result.Conversation = conversation
	return result, nil
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
	templateKind := promptKindForTarget(conversation.TargetType)
	template, ok := u.promptStore.Get(templateKind)
	if !ok {
		return nil, conversationdomain.Message{}, fmt.Errorf("prompt template %q not found", templateKind)
	}

	systemPrompt, userPrompt, err := template.Render(buildPromptData(conversation, target, latestUserMessage))
	if err != nil {
		return nil, conversationdomain.Message{}, fmt.Errorf("render prompt template %q: %w", templateKind, err)
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
		return "Confirmed the latest project suggestion and applied it to the project."
	case conversationdomain.TargetTypeAsset:
		return "Confirmed the latest asset suggestion and applied it to the asset."
	default:
		return "Confirmed the latest suggestion."
	}
}

func promptKindForTarget(targetType string) string {
	switch targetType {
	case conversationdomain.TargetTypeProject:
		return projectRefinementPromptKind
	case conversationdomain.TargetTypeAsset:
		return assetRefinementPromptKind
	default:
		return ""
	}
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
