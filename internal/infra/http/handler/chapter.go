package handler

import (
	"context"
	"strings"

	chapterdomain "novelforge/backend/internal/domain/chapter"
	generationdomain "novelforge/backend/internal/domain/generation"
	"novelforge/backend/internal/infra/http/middleware"
	chapterservice "novelforge/backend/internal/service/chapter"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/google/uuid"
)

// ChapterHandler 处理章节(chapter) HTTP 请求。
type ChapterHandler struct {
	useCase chapterservice.UseCase
}

// NewChapterHandler 创建章节(chapter) HTTP 处理程序。
func NewChapterHandler(useCase chapterservice.UseCase) *ChapterHandler {
	return &ChapterHandler{useCase: useCase}
}

type chapterGenerateRequest struct {
	Title       string `json:"title"`
	Ordinal     int    `json:"ordinal"`
	Instruction string `json:"instruction"`
}

type chapterContinueRequest struct {
	Instruction string `json:"instruction"`
}

type chapterRewriteRequest struct {
	TargetText  string `json:"target_text"`
	Instruction string `json:"instruction"`
}

type chapterResponse struct {
	ID                      string  `json:"id"`
	ProjectID               string  `json:"project_id"`
	Title                   string  `json:"title"`
	Ordinal                 int     `json:"ordinal"`
	Status                  string  `json:"status"`
	Content                 string  `json:"content"`
	CurrentDraftID          string  `json:"current_draft_id,omitempty"`
	CurrentDraftConfirmedAt *string `json:"current_draft_confirmed_at,omitempty"`
	CurrentDraftConfirmedBy string  `json:"current_draft_confirmed_by,omitempty"`
	CreatedAt               string  `json:"created_at"`
	UpdatedAt               string  `json:"updated_at"`
}

type chapterListResponse struct {
	Chapters []chapterResponse `json:"chapters"`
}

type generationRecordResponse struct {
	ID               string `json:"id"`
	ProjectID        string `json:"project_id"`
	ChapterID        string `json:"chapter_id,omitempty"`
	ConversationID   string `json:"conversation_id,omitempty"`
	Kind             string `json:"kind"`
	Status           string `json:"status"`
	InputSnapshotRef string `json:"input_snapshot_ref"`
	OutputRef        string `json:"output_ref"`
	TokenUsage       int    `json:"token_usage"`
	DurationMillis   int64  `json:"duration_millis"`
	ErrorMessage     string `json:"error_message,omitempty"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

type chapterGenerationResponse struct {
	Chapter          chapterResponse          `json:"chapter"`
	GenerationRecord generationRecordResponse `json:"generation_record"`
}

func (h *ChapterHandler) Create(c context.Context, ctx *app.RequestContext) {
	var request chapterGenerateRequest
	if err := ctx.BindJSON(&request); err != nil {
		writeError(ctx, consts.StatusBadRequest, err.Error())
		return
	}

	result, err := h.useCase.Generate(c, chapterservice.GenerateParams{
		ProjectID:   ctx.Param("projectID"),
		Title:       request.Title,
		Ordinal:     request.Ordinal,
		Instruction: request.Instruction,
	})
	if err != nil {
		writeServiceError(ctx, err)
		return
	}

	ctx.JSON(consts.StatusCreated, newChapterGenerationResponse(result.Chapter, result.GenerationRecord))
}

func (h *ChapterHandler) ListByProject(c context.Context, ctx *app.RequestContext) {
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

	items, err := h.useCase.ListByProject(c, chapterdomain.ListByProjectParams{
		ProjectID: ctx.Param("projectID"),
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		writeServiceError(ctx, err)
		return
	}

	ctx.JSON(consts.StatusOK, chapterListResponse{Chapters: newChapterResponses(items)})
}

func (h *ChapterHandler) GetByID(c context.Context, ctx *app.RequestContext) {
	chapter, err := h.useCase.GetByID(c, ctx.Param("chapterID"))
	if err != nil {
		writeServiceError(ctx, err)
		return
	}

	ctx.JSON(consts.StatusOK, newChapterResponse(chapter))
}

func (h *ChapterHandler) Continue(c context.Context, ctx *app.RequestContext) {
	var request chapterContinueRequest
	if err := ctx.BindJSON(&request); err != nil {
		writeError(ctx, consts.StatusBadRequest, err.Error())
		return
	}

	result, err := h.useCase.Continue(c, chapterservice.ContinueParams{
		ChapterID:   ctx.Param("chapterID"),
		Instruction: request.Instruction,
	})
	if err != nil {
		writeServiceError(ctx, err)
		return
	}

	ctx.JSON(consts.StatusOK, newChapterGenerationResponse(result.Chapter, result.GenerationRecord))
}

func (h *ChapterHandler) Rewrite(c context.Context, ctx *app.RequestContext) {
	var request chapterRewriteRequest
	if err := ctx.BindJSON(&request); err != nil {
		writeError(ctx, consts.StatusBadRequest, err.Error())
		return
	}

	result, err := h.useCase.Rewrite(c, chapterservice.RewriteParams{
		ChapterID:   ctx.Param("chapterID"),
		TargetText:  request.TargetText,
		Instruction: request.Instruction,
	})
	if err != nil {
		writeServiceError(ctx, err)
		return
	}

	ctx.JSON(consts.StatusOK, newChapterGenerationResponse(result.Chapter, result.GenerationRecord))
}

func (h *ChapterHandler) Confirm(c context.Context, ctx *app.RequestContext) {
	userID, ok := currentUserID(ctx)
	if !ok {
		writeError(ctx, consts.StatusUnauthorized, "user_id must be a valid UUID")
		return
	}

	chapter, err := h.useCase.Confirm(c, chapterservice.ConfirmParams{
		ChapterID:   ctx.Param("chapterID"),
		ConfirmedBy: userID,
	})
	if err != nil {
		writeServiceError(ctx, err)
		return
	}

	ctx.JSON(consts.StatusOK, newChapterResponse(chapter))
}

func (h *ChapterHandler) CreateStream(c context.Context, ctx *app.RequestContext) {
	var request chapterGenerateRequest
	if err := ctx.BindJSON(&request); err != nil {
		writeError(ctx, consts.StatusBadRequest, err.Error())
		return
	}

	result, err := h.useCase.GenerateStream(c, chapterservice.GenerateParams{
		ProjectID:   ctx.Param("projectID"),
		Title:       request.Title,
		Ordinal:     request.Ordinal,
		Instruction: request.Instruction,
	})
	if err != nil {
		writeServiceError(ctx, err)
		return
	}

	writeSSEStream(ctx, result.Stream, func(content string) (any, error) {
		generateResult, err := result.OnComplete(content)
		if err != nil {
			return nil, err
		}
		return newChapterGenerationResponse(generateResult.Chapter, generateResult.GenerationRecord), nil
	})
}

func (h *ChapterHandler) ContinueStream(c context.Context, ctx *app.RequestContext) {
	var request chapterContinueRequest
	if err := ctx.BindJSON(&request); err != nil {
		writeError(ctx, consts.StatusBadRequest, err.Error())
		return
	}

	result, err := h.useCase.ContinueStream(c, chapterservice.ContinueParams{
		ChapterID:   ctx.Param("chapterID"),
		Instruction: request.Instruction,
	})
	if err != nil {
		writeServiceError(ctx, err)
		return
	}

	writeSSEStream(ctx, result.Stream, func(content string) (any, error) {
		continueResult, err := result.OnComplete(content)
		if err != nil {
			return nil, err
		}
		return newChapterGenerationResponse(continueResult.Chapter, continueResult.GenerationRecord), nil
	})
}

func (h *ChapterHandler) RewriteStream(c context.Context, ctx *app.RequestContext) {
	var request chapterRewriteRequest
	if err := ctx.BindJSON(&request); err != nil {
		writeError(ctx, consts.StatusBadRequest, err.Error())
		return
	}

	result, err := h.useCase.RewriteStream(c, chapterservice.RewriteParams{
		ChapterID:   ctx.Param("chapterID"),
		TargetText:  request.TargetText,
		Instruction: request.Instruction,
	})
	if err != nil {
		writeServiceError(ctx, err)
		return
	}

	writeSSEStream(ctx, result.Stream, func(content string) (any, error) {
		rewriteResult, err := result.OnComplete(content)
		if err != nil {
			return nil, err
		}
		return newChapterGenerationResponse(rewriteResult.Chapter, rewriteResult.GenerationRecord), nil
	})
}

func currentUserID(ctx *app.RequestContext) (string, bool) {
	value, exists := ctx.Get(middleware.UserIDContextKey)
	if !exists {
		return "", false
	}
	userID, ok := value.(string)
	if !ok {
		return "", false
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "", false
	}
	if _, err := uuid.Parse(userID); err != nil {
		return "", false
	}
	return userID, true
}

func newChapterGenerationResponse(chapter *chapterdomain.Chapter, record *generationdomain.GenerationRecord) chapterGenerationResponse {
	return chapterGenerationResponse{
		Chapter:          newChapterResponse(chapter),
		GenerationRecord: newGenerationRecordResponse(record),
	}
}

func newChapterResponse(entity *chapterdomain.Chapter) chapterResponse {
	response := chapterResponse{
		ID:             entity.ID,
		ProjectID:      entity.ProjectID,
		Title:          entity.Title,
		Ordinal:        entity.Ordinal,
		Status:         entity.Status,
		Content:        entity.Content,
		CurrentDraftID: entity.CurrentDraftID,
		CreatedAt:      entity.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:      entity.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
	if entity.CurrentDraftConfirmedAt != nil {
		formatted := entity.CurrentDraftConfirmedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
		response.CurrentDraftConfirmedAt = &formatted
	}
	response.CurrentDraftConfirmedBy = entity.CurrentDraftConfirmedBy
	return response
}

func newChapterResponses(entities []*chapterdomain.Chapter) []chapterResponse {
	responses := make([]chapterResponse, 0, len(entities))
	for _, entity := range entities {
		responses = append(responses, newChapterResponse(entity))
	}
	return responses
}

func newGenerationRecordResponse(entity *generationdomain.GenerationRecord) generationRecordResponse {
	return generationRecordResponse{
		ID:               entity.ID,
		ProjectID:        entity.ProjectID,
		ChapterID:        entity.ChapterID,
		ConversationID:   entity.ConversationID,
		Kind:             entity.Kind,
		Status:           entity.Status,
		InputSnapshotRef: entity.InputSnapshotRef,
		OutputRef:        entity.OutputRef,
		TokenUsage:       entity.TokenUsage,
		DurationMillis:   entity.DurationMillis,
		ErrorMessage:     entity.ErrorMessage,
		CreatedAt:        entity.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:        entity.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}
