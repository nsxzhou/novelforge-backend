package handler

import (
	"context"

	assetdomain "novelforge/backend/internal/domain/asset"
	generationdomain "novelforge/backend/internal/domain/generation"
	assetservice "novelforge/backend/internal/service/asset"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// AssetHandler 处理资产(asset)的 HTTP 请求。
type AssetHandler struct {
	useCase assetservice.UseCase
}

// NewAssetHandler 创建资产(asset) HTTP 处理程序。
func NewAssetHandler(useCase assetservice.UseCase) *AssetHandler {
	return &AssetHandler{useCase: useCase}
}

type assetUpsertRequest struct {
	Type    string `json:"type"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

type assetGenerateRequest struct {
	Type        string `json:"type"`
	Instruction string `json:"instruction"`
}

type assetResponse struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type assetListResponse struct {
	Assets []assetResponse `json:"assets"`
}

type assetGenerationResponse struct {
	Asset            assetResponse            `json:"asset"`
	GenerationRecord generationRecordResponse `json:"generation_record"`
}

func (h *AssetHandler) Create(c context.Context, ctx *app.RequestContext) {
	var request assetUpsertRequest
	if err := ctx.BindJSON(&request); err != nil {
		writeError(ctx, consts.StatusBadRequest, err.Error())
		return
	}

	entity := &assetdomain.Asset{
		ProjectID: ctx.Param("projectID"),
		Type:      request.Type,
		Title:     request.Title,
		Content:   request.Content,
	}
	if err := h.useCase.Create(c, entity); err != nil {
		writeServiceError(ctx, err)
		return
	}

	ctx.JSON(consts.StatusCreated, newAssetResponse(entity))
}

func (h *AssetHandler) ListByProject(c context.Context, ctx *app.RequestContext) {
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

	assetType, hasType, err := parseOptionalStringQuery(ctx, "type")
	if err != nil {
		writeError(ctx, consts.StatusBadRequest, err.Error())
		return
	}

	var entities []*assetdomain.Asset
	if hasType {
		entities, err = h.useCase.ListByProjectAndType(c, assetdomain.ListByProjectAndTypeParams{
			ProjectID: ctx.Param("projectID"),
			Type:      assetType,
			Limit:     limit,
			Offset:    offset,
		})
	} else {
		entities, err = h.useCase.ListByProject(c, assetdomain.ListByProjectParams{
			ProjectID: ctx.Param("projectID"),
			Limit:     limit,
			Offset:    offset,
		})
	}
	if err != nil {
		writeServiceError(ctx, err)
		return
	}

	ctx.JSON(consts.StatusOK, assetListResponse{Assets: newAssetResponses(entities)})
}

func (h *AssetHandler) Generate(c context.Context, ctx *app.RequestContext) {
	var request assetGenerateRequest
	if err := ctx.BindJSON(&request); err != nil {
		writeError(ctx, consts.StatusBadRequest, err.Error())
		return
	}

	result, err := h.useCase.Generate(c, assetservice.GenerateParams{
		ProjectID:   ctx.Param("projectID"),
		Type:        request.Type,
		Instruction: request.Instruction,
	})
	if err != nil {
		writeServiceError(ctx, err)
		return
	}

	ctx.JSON(consts.StatusCreated, newAssetGenerationResponse(result.Asset, result.GenerationRecord))
}

func (h *AssetHandler) GenerateStream(c context.Context, ctx *app.RequestContext) {
	var request assetGenerateRequest
	if err := ctx.BindJSON(&request); err != nil {
		writeError(ctx, consts.StatusBadRequest, err.Error())
		return
	}

	result, err := h.useCase.GenerateStream(c, assetservice.GenerateParams{
		ProjectID:   ctx.Param("projectID"),
		Type:        request.Type,
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
		return newAssetGenerationResponse(generateResult.Asset, generateResult.GenerationRecord), nil
	})
}

func (h *AssetHandler) GetByID(c context.Context, ctx *app.RequestContext) {
	entity, err := h.useCase.GetByID(c, ctx.Param("assetID"))
	if err != nil {
		writeServiceError(ctx, err)
		return
	}

	ctx.JSON(consts.StatusOK, newAssetResponse(entity))
}

func (h *AssetHandler) Update(c context.Context, ctx *app.RequestContext) {
	var request assetUpsertRequest
	if err := ctx.BindJSON(&request); err != nil {
		writeError(ctx, consts.StatusBadRequest, err.Error())
		return
	}

	entity := &assetdomain.Asset{
		ID:      ctx.Param("assetID"),
		Type:    request.Type,
		Title:   request.Title,
		Content: request.Content,
	}
	if err := h.useCase.Update(c, entity); err != nil {
		writeServiceError(ctx, err)
		return
	}

	ctx.JSON(consts.StatusOK, newAssetResponse(entity))
}

func (h *AssetHandler) Delete(c context.Context, ctx *app.RequestContext) {
	if err := h.useCase.Delete(c, ctx.Param("assetID")); err != nil {
		writeServiceError(ctx, err)
		return
	}

	ctx.SetStatusCode(consts.StatusNoContent)
}

func newAssetResponse(entity *assetdomain.Asset) assetResponse {
	return assetResponse{
		ID:        entity.ID,
		ProjectID: entity.ProjectID,
		Type:      entity.Type,
		Title:     entity.Title,
		Content:   entity.Content,
		CreatedAt: entity.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: entity.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

func newAssetResponses(entities []*assetdomain.Asset) []assetResponse {
	responses := make([]assetResponse, 0, len(entities))
	for _, entity := range entities {
		responses = append(responses, newAssetResponse(entity))
	}
	return responses
}

func newAssetGenerationResponse(asset *assetdomain.Asset, record *generationdomain.GenerationRecord) assetGenerationResponse {
	return assetGenerationResponse{
		Asset:            newAssetResponse(asset),
		GenerationRecord: newGenerationRecordResponse(record),
	}
}
