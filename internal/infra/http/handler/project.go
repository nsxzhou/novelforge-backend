package handler

import (
	"context"

	projectdomain "inkmuse/backend/internal/domain/project"
	projectservice "inkmuse/backend/internal/service/project"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// ProjectHandler 处理项目(project)的 HTTP 请求。
type ProjectHandler struct {
	useCase projectservice.UseCase
}

// NewProjectHandler 创建项目(project) HTTP 处理程序。
func NewProjectHandler(useCase projectservice.UseCase) *ProjectHandler {
	return &ProjectHandler{useCase: useCase}
}

type projectUpsertRequest struct {
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Status  string `json:"status"`
}

type projectResponse struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Summary   string `json:"summary"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type projectListResponse struct {
	Projects []projectResponse `json:"projects"`
}

func (h *ProjectHandler) Create(c context.Context, ctx *app.RequestContext) {
	var request projectUpsertRequest
	if err := ctx.BindJSON(&request); err != nil {
		writeError(ctx, consts.StatusBadRequest, err.Error())
		return
	}

	entity := &projectdomain.Project{
		Title:   request.Title,
		Summary: request.Summary,
		Status:  request.Status,
	}
	if err := h.useCase.Create(c, entity); err != nil {
		writeServiceError(ctx, err)
		return
	}

	ctx.JSON(consts.StatusCreated, newProjectResponse(entity))
}

func (h *ProjectHandler) List(c context.Context, ctx *app.RequestContext) {
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

	status, hasStatus, err := parseOptionalStringQuery(ctx, "status")
	if err != nil {
		writeError(ctx, consts.StatusBadRequest, err.Error())
		return
	}

	params := projectdomain.ListParams{
		Limit:  limit,
		Offset: offset,
	}
	if hasStatus {
		params.Status = status
	}

	entities, listErr := h.useCase.List(c, params)
	if listErr != nil {
		writeServiceError(ctx, listErr)
		return
	}

	ctx.JSON(consts.StatusOK, projectListResponse{Projects: newProjectResponses(entities)})
}

func (h *ProjectHandler) GetByID(c context.Context, ctx *app.RequestContext) {
	entity, err := h.useCase.GetByID(c, ctx.Param("projectID"))
	if err != nil {
		writeServiceError(ctx, err)
		return
	}

	ctx.JSON(consts.StatusOK, newProjectResponse(entity))
}

func (h *ProjectHandler) Update(c context.Context, ctx *app.RequestContext) {
	var request projectUpsertRequest
	if err := ctx.BindJSON(&request); err != nil {
		writeError(ctx, consts.StatusBadRequest, err.Error())
		return
	}

	entity := &projectdomain.Project{
		ID:      ctx.Param("projectID"),
		Title:   request.Title,
		Summary: request.Summary,
		Status:  request.Status,
	}
	if err := h.useCase.Update(c, entity); err != nil {
		writeServiceError(ctx, err)
		return
	}

	ctx.JSON(consts.StatusOK, newProjectResponse(entity))
}

func newProjectResponse(entity *projectdomain.Project) projectResponse {
	return projectResponse{
		ID:        entity.ID,
		Title:     entity.Title,
		Summary:   entity.Summary,
		Status:    entity.Status,
		CreatedAt: entity.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: entity.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

func newProjectResponses(entities []*projectdomain.Project) []projectResponse {
	responses := make([]projectResponse, 0, len(entities))
	for _, entity := range entities {
		responses = append(responses, newProjectResponse(entity))
	}
	return responses
}
