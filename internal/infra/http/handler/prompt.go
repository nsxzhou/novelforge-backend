package handler

import (
	"context"
	"fmt"
	"strings"

	promptdomain "inkmuse/backend/internal/domain/prompt"
	"inkmuse/backend/internal/infra/llm/prompts"
	"inkmuse/backend/pkg/config"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/google/uuid"
)

// PromptHandler 处理项目级 prompt 模板覆盖的 HTTP 请求。
type PromptHandler struct {
	overrides   promptdomain.OverrideRepository
	promptStore *prompts.Store
}

// NewPromptHandler 创建项目级 prompt HTTP 处理程序。
func NewPromptHandler(overrides promptdomain.OverrideRepository, promptStore *prompts.Store) *PromptHandler {
	return &PromptHandler{
		overrides:   overrides,
		promptStore: promptStore,
	}
}

type promptTemplateResponse struct {
	Capability         string   `json:"capability"`
	System             string   `json:"system"`
	User               string   `json:"user"`
	IsOverride         bool     `json:"is_override"`
	AvailableVariables []string `json:"available_variables"`
}

type promptListResponse struct {
	Prompts []promptTemplateResponse `json:"prompts"`
}

type promptUpsertRequest struct {
	System string `json:"system"`
	User   string `json:"user"`
}

// capabilityVariables 定义每个 capability 可用的模板变量。
var capabilityVariables = map[config.PromptCapability][]string{
	config.PromptCapabilityChapterGeneration: {
		"ProjectTitle", "ProjectSummary", "ChapterTitle", "ChapterOrdinal",
		"OutlineContext", "WorldbuildingContext", "CharacterContext", "Instruction",
	},
	config.PromptCapabilityChapterContinuation: {
		"ProjectTitle", "ProjectSummary", "ChapterTitle", "ChapterOrdinal",
		"OutlineContext", "WorldbuildingContext", "CharacterContext",
		"CurrentChapterContent", "Instruction",
	},
	config.PromptCapabilityChapterRewrite: {
		"ProjectTitle", "ProjectSummary", "ChapterTitle", "ChapterOrdinal",
		"OutlineContext", "WorldbuildingContext", "CharacterContext",
		"CurrentChapterContent", "TargetText", "Instruction",
	},
	config.PromptCapabilityAssetGeneration: {
		"ProjectTitle", "ProjectSummary", "AssetType", "Instruction",
	},
	config.PromptCapabilityProjectRefinement: {
		"ProjectTitle", "ProjectSummary", "ConversationHistory", "LatestUserMessage",
	},
	config.PromptCapabilityAssetRefinement: {
		"AssetTitle", "AssetContent", "AssetType", "ConversationHistory", "LatestUserMessage",
	},
}

func (h *PromptHandler) List(c context.Context, ctx *app.RequestContext) {
	projectID := ctx.Param("projectID")
	if err := validateProjectUUID(projectID); err != nil {
		writeError(ctx, consts.StatusBadRequest, err.Error())
		return
	}

	overrides, err := h.overrides.ListByProject(c, projectID)
	if err != nil {
		writeError(ctx, consts.StatusInternalServerError, "internal server error")
		return
	}

	overrideMap := make(map[string]*promptdomain.ProjectPromptOverride, len(overrides))
	for _, o := range overrides {
		overrideMap[o.Capability] = o
	}

	defaults := h.promptStore.List()
	capabilities := config.AllPromptCapabilities()
	responses := make([]promptTemplateResponse, 0, len(capabilities))

	for _, cap := range capabilities {
		capStr := string(cap)
		vars := capabilityVariables[cap]
		if vars == nil {
			vars = []string{}
		}

		if override, ok := overrideMap[capStr]; ok {
			responses = append(responses, promptTemplateResponse{
				Capability:         capStr,
				System:             override.System,
				User:               override.User,
				IsOverride:         true,
				AvailableVariables: vars,
			})
		} else if snapshot, ok := defaults[cap]; ok {
			responses = append(responses, promptTemplateResponse{
				Capability:         capStr,
				System:             snapshot.System,
				User:               snapshot.User,
				IsOverride:         false,
				AvailableVariables: vars,
			})
		}
	}

	ctx.JSON(consts.StatusOK, promptListResponse{Prompts: responses})
}

func (h *PromptHandler) Get(c context.Context, ctx *app.RequestContext) {
	projectID := ctx.Param("projectID")
	capability := ctx.Param("capability")

	if err := validateProjectUUID(projectID); err != nil {
		writeError(ctx, consts.StatusBadRequest, err.Error())
		return
	}
	cap := config.PromptCapability(capability)
	if !isValidCapability(cap) {
		writeError(ctx, consts.StatusBadRequest, fmt.Sprintf("unsupported capability %q", capability))
		return
	}

	vars := capabilityVariables[cap]
	if vars == nil {
		vars = []string{}
	}

	override, err := h.overrides.GetByProjectAndCapability(c, projectID, capability)
	if err == nil && override != nil {
		ctx.JSON(consts.StatusOK, promptTemplateResponse{
			Capability:         capability,
			System:             override.System,
			User:               override.User,
			IsOverride:         true,
			AvailableVariables: vars,
		})
		return
	}

	snapshot, ok := h.promptStore.List()[cap]
	if !ok {
		writeError(ctx, consts.StatusNotFound, fmt.Sprintf("prompt template %q not found", capability))
		return
	}

	ctx.JSON(consts.StatusOK, promptTemplateResponse{
		Capability:         capability,
		System:             snapshot.System,
		User:               snapshot.User,
		IsOverride:         false,
		AvailableVariables: vars,
	})
}

func (h *PromptHandler) Upsert(c context.Context, ctx *app.RequestContext) {
	projectID := ctx.Param("projectID")
	capability := ctx.Param("capability")

	if err := validateProjectUUID(projectID); err != nil {
		writeError(ctx, consts.StatusBadRequest, err.Error())
		return
	}
	cap := config.PromptCapability(capability)
	if !isValidCapability(cap) {
		writeError(ctx, consts.StatusBadRequest, fmt.Sprintf("unsupported capability %q", capability))
		return
	}

	var request promptUpsertRequest
	if err := ctx.BindJSON(&request); err != nil {
		writeError(ctx, consts.StatusBadRequest, err.Error())
		return
	}

	request.System = strings.TrimSpace(request.System)
	request.User = strings.TrimSpace(request.User)
	if request.System == "" {
		writeError(ctx, consts.StatusBadRequest, "system must not be empty")
		return
	}
	if request.User == "" {
		writeError(ctx, consts.StatusBadRequest, "user must not be empty")
		return
	}

	// 验证模板语法合法性
	if _, err := prompts.ParseTemplate(capability, request.System, request.User); err != nil {
		writeError(ctx, consts.StatusBadRequest, fmt.Sprintf("invalid template: %s", err.Error()))
		return
	}

	override := &promptdomain.ProjectPromptOverride{
		ProjectID:  projectID,
		Capability: capability,
		System:     request.System,
		User:       request.User,
	}
	if err := h.overrides.Upsert(c, override); err != nil {
		writeError(ctx, consts.StatusInternalServerError, "internal server error")
		return
	}

	vars := capabilityVariables[cap]
	if vars == nil {
		vars = []string{}
	}

	ctx.JSON(consts.StatusOK, promptTemplateResponse{
		Capability:         capability,
		System:             override.System,
		User:               override.User,
		IsOverride:         true,
		AvailableVariables: vars,
	})
}

func (h *PromptHandler) Delete(c context.Context, ctx *app.RequestContext) {
	projectID := ctx.Param("projectID")
	capability := ctx.Param("capability")

	if err := validateProjectUUID(projectID); err != nil {
		writeError(ctx, consts.StatusBadRequest, err.Error())
		return
	}
	cap := config.PromptCapability(capability)
	if !isValidCapability(cap) {
		writeError(ctx, consts.StatusBadRequest, fmt.Sprintf("unsupported capability %q", capability))
		return
	}

	if err := h.overrides.Delete(c, projectID, capability); err != nil {
		writeError(ctx, consts.StatusNotFound, fmt.Sprintf("no override found for capability %q", capability))
		return
	}

	ctx.SetStatusCode(consts.StatusNoContent)
}

func validateProjectUUID(id string) error {
	if _, err := uuid.Parse(strings.TrimSpace(id)); err != nil {
		return fmt.Errorf("project_id must be a valid UUID")
	}
	return nil
}

func isValidCapability(cap config.PromptCapability) bool {
	for _, c := range config.AllPromptCapabilities() {
		if c == cap {
			return true
		}
	}
	return false
}
