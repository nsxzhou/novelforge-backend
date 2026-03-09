package handler

import (
	"context"

	assetdomain "novelforge/backend/internal/domain/asset"
	conversationdomain "novelforge/backend/internal/domain/conversation"
	projectdomain "novelforge/backend/internal/domain/project"
	conversationservice "novelforge/backend/internal/service/conversation"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// ConversationHandler 处理对话(conversation)细化 HTTP 请求。
type ConversationHandler struct {
	useCase conversationservice.UseCase
}

// NewConversationHandler 创建对话(conversation) HTTP 处理程序。
func NewConversationHandler(useCase conversationservice.UseCase) *ConversationHandler {
	return &ConversationHandler{useCase: useCase}
}

type conversationStartRequest struct {
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	Message    string `json:"message"`
}

type conversationReplyRequest struct {
	Message string `json:"message"`
}

type conversationMessageResponse struct {
	ID        string `json:"id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

type pendingSuggestionResponse struct {
	Title   string `json:"title,omitempty"`
	Summary string `json:"summary,omitempty"`
	Content string `json:"content,omitempty"`
}

type conversationResponse struct {
	ID                string                    `json:"id"`
	ProjectID         string                    `json:"project_id"`
	TargetType        string                    `json:"target_type"`
	TargetID          string                    `json:"target_id"`
	Messages          []conversationMessageResponse `json:"messages"`
	PendingSuggestion *pendingSuggestionResponse `json:"pending_suggestion"`
	CreatedAt         string                    `json:"created_at"`
	UpdatedAt         string                    `json:"updated_at"`
}

type conversationListResponse struct {
	Conversations []conversationResponse `json:"conversations"`
}

type confirmConversationResponse struct {
	Conversation conversationResponse `json:"conversation"`
	Project      *projectResponse     `json:"project,omitempty"`
	Asset        *assetResponse       `json:"asset,omitempty"`
}

func (h *ConversationHandler) Start(c context.Context, ctx *app.RequestContext) {
	var request conversationStartRequest
	if err := ctx.BindJSON(&request); err != nil {
		writeError(ctx, consts.StatusBadRequest, err.Error())
		return
	}

	conversation, err := h.useCase.Start(c, conversationservice.StartParams{
		ProjectID:  ctx.Param("projectID"),
		TargetType: request.TargetType,
		TargetID:   request.TargetID,
		Message:    request.Message,
	})
	if err != nil {
		writeServiceError(ctx, err)
		return
	}

	ctx.JSON(consts.StatusCreated, newConversationResponse(conversation))
}

func (h *ConversationHandler) Reply(c context.Context, ctx *app.RequestContext) {
	var request conversationReplyRequest
	if err := ctx.BindJSON(&request); err != nil {
		writeError(ctx, consts.StatusBadRequest, err.Error())
		return
	}

	conversation, err := h.useCase.Reply(c, conversationservice.ReplyParams{
		ConversationID: ctx.Param("conversationID"),
		Message:        request.Message,
	})
	if err != nil {
		writeServiceError(ctx, err)
		return
	}

	ctx.JSON(consts.StatusOK, newConversationResponse(conversation))
}

func (h *ConversationHandler) Confirm(c context.Context, ctx *app.RequestContext) {
	result, err := h.useCase.Confirm(c, ctx.Param("conversationID"))
	if err != nil {
		writeServiceError(ctx, err)
		return
	}

	response := confirmConversationResponse{Conversation: newConversationResponse(result.Conversation)}
	response.Project = newConfirmProjectResponse(result.Project)
	response.Asset = newConfirmAssetResponse(result.Asset)
	ctx.JSON(consts.StatusOK, response)
}

func (h *ConversationHandler) GetByID(c context.Context, ctx *app.RequestContext) {
	conversation, err := h.useCase.GetByID(c, ctx.Param("conversationID"))
	if err != nil {
		writeServiceError(ctx, err)
		return
	}

	ctx.JSON(consts.StatusOK, newConversationResponse(conversation))
}

func (h *ConversationHandler) List(c context.Context, ctx *app.RequestContext) {
	limit, err := parseNonNegativeIntQuery(ctx, "limit")
	if err != nil {
		writeError(ctx, consts.StatusBadRequest, err.Error())
		return
	}
	offset, err := parseNonNegativeIntQuery(ctx, "offset")
	if err != nil {
		writeError(ctx, consts.StatusBadRequest, err.Error())
		return
	}
	targetType, hasTargetType, err := parseOptionalStringQuery(ctx, "target_type")
	if err != nil {
		writeError(ctx, consts.StatusBadRequest, err.Error())
		return
	}
	targetID, hasTargetID, err := parseOptionalStringQuery(ctx, "target_id")
	if err != nil {
		writeError(ctx, consts.StatusBadRequest, err.Error())
		return
	}

	params := conversationservice.ListParams{
		ProjectID: ctx.Param("projectID"),
		Limit:     limit,
		Offset:    offset,
	}
	if hasTargetType {
		params.TargetType = targetType
	}
	if hasTargetID {
		params.TargetID = targetID
	}

	items, err := h.useCase.List(c, params)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}

	ctx.JSON(consts.StatusOK, conversationListResponse{Conversations: newConversationResponses(items)})
}

func newConversationResponse(entity *conversationdomain.Conversation) conversationResponse {
	return conversationResponse{
		ID:                entity.ID,
		ProjectID:         entity.ProjectID,
		TargetType:        entity.TargetType,
		TargetID:          entity.TargetID,
		Messages:          newConversationMessageResponses(entity.Messages),
		PendingSuggestion: newPendingSuggestionResponse(entity.PendingSuggestion),
		CreatedAt:         entity.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:         entity.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

func newConversationResponses(entities []*conversationdomain.Conversation) []conversationResponse {
	responses := make([]conversationResponse, 0, len(entities))
	for _, entity := range entities {
		responses = append(responses, newConversationResponse(entity))
	}
	return responses
}

func newConversationMessageResponses(messages []conversationdomain.Message) []conversationMessageResponse {
	responses := make([]conversationMessageResponse, 0, len(messages))
	for _, message := range messages {
		responses = append(responses, conversationMessageResponse{
			ID:        message.ID,
			Role:      message.Role,
			Content:   message.Content,
			CreatedAt: message.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	return responses
}

func newPendingSuggestionResponse(suggestion *conversationdomain.PendingSuggestion) *pendingSuggestionResponse {
	if suggestion == nil {
		return nil
	}
	return &pendingSuggestionResponse{
		Title:   suggestion.Title,
		Summary: suggestion.Summary,
		Content: suggestion.Content,
	}
}

func newConfirmProjectResponse(entity *projectdomain.Project) *projectResponse {
	if entity == nil {
		return nil
	}
	response := newProjectResponse(entity)
	return &response
}

func newConfirmAssetResponse(entity *assetdomain.Asset) *assetResponse {
	if entity == nil {
		return nil
	}
	response := newAssetResponse(entity)
	return &response
}
