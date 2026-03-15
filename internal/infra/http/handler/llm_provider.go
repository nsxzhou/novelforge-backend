package handler

import (
	"context"

	"novelforge/backend/internal/domain/llmprovider"
	"novelforge/backend/internal/infra/llm"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// LLMProviderHandler handles user self-service operations for LLM provider management.
type LLMProviderHandler struct {
	registry *llm.Registry
	repo     llmprovider.Repository
}

// NewLLMProviderHandler creates an LLM provider HTTP handler.
func NewLLMProviderHandler(registry *llm.Registry, repo llmprovider.Repository) *LLMProviderHandler {
	return &LLMProviderHandler{registry: registry, repo: repo}
}

type providerResponse struct {
	ID         string `json:"id"`
	Provider   string `json:"provider"`
	Model      string `json:"model"`
	BaseURL    string `json:"base_url"`
	APIKey     string `json:"api_key"`
	TimeoutSec int    `json:"timeout_seconds"`
	Priority   int    `json:"priority"`
	Enabled    bool   `json:"enabled"`
}

type providerListResponse struct {
	Providers []providerResponse `json:"providers"`
}

type addProviderRequest struct {
	ID         string `json:"id"`
	Provider   string `json:"provider"`
	Model      string `json:"model"`
	BaseURL    string `json:"base_url"`
	APIKey     string `json:"api_key"`
	TimeoutSec int    `json:"timeout_seconds"`
	Priority   int    `json:"priority"`
	Enabled    bool   `json:"enabled"`
}

type updateProviderRequest struct {
	Provider   string `json:"provider"`
	Model      string `json:"model"`
	BaseURL    string `json:"base_url"`
	APIKey     string `json:"api_key"`
	TimeoutSec int    `json:"timeout_seconds"`
	Priority   int    `json:"priority"`
	Enabled    bool   `json:"enabled"`
}

// ListProviders returns all registered LLM providers with masked API keys.
func (h *LLMProviderHandler) ListProviders(_ context.Context, ctx *app.RequestContext) {
	configs := h.registry.ListProviders()
	responses := make([]providerResponse, 0, len(configs))
	for _, cfg := range configs {
		responses = append(responses, toProviderResponse(cfg))
	}
	ctx.JSON(consts.StatusOK, providerListResponse{Providers: responses})
}

// AddProvider adds a new LLM provider to the registry and persists it to the database.
func (h *LLMProviderHandler) AddProvider(c context.Context, ctx *app.RequestContext) {
	var req addProviderRequest
	if err := ctx.BindJSON(&req); err != nil {
		writeError(ctx, consts.StatusBadRequest, err.Error())
		return
	}

	cfg := llm.ProviderConfig{
		ID:         req.ID,
		Provider:   req.Provider,
		Model:      req.Model,
		BaseURL:    req.BaseURL,
		APIKey:     req.APIKey,
		TimeoutSec: req.TimeoutSec,
		Priority:   req.Priority,
		Enabled:    req.Enabled,
	}

	if err := h.registry.AddProvider(cfg); err != nil {
		writeError(ctx, consts.StatusBadRequest, err.Error())
		return
	}

	if err := h.repo.Upsert(c, llm.ProviderConfigToDomain(cfg)); err != nil {
		// Registry was updated but DB failed; remove from registry to stay consistent.
		_ = h.registry.RemoveProvider(cfg.ID)
		writeError(ctx, consts.StatusInternalServerError, "failed to persist provider")
		return
	}

	ctx.JSON(consts.StatusCreated, toProviderResponse(cfg))
}

// UpdateProvider updates an existing LLM provider's configuration and persists the change.
func (h *LLMProviderHandler) UpdateProvider(c context.Context, ctx *app.RequestContext) {
	providerID := ctx.Param("providerID")

	var req updateProviderRequest
	if err := ctx.BindJSON(&req); err != nil {
		writeError(ctx, consts.StatusBadRequest, err.Error())
		return
	}

	cfg := llm.ProviderConfig{
		ID:         providerID,
		Provider:   req.Provider,
		Model:      req.Model,
		BaseURL:    req.BaseURL,
		APIKey:     req.APIKey,
		TimeoutSec: req.TimeoutSec,
		Priority:   req.Priority,
		Enabled:    req.Enabled,
	}

	if err := h.registry.UpdateProvider(cfg); err != nil {
		writeError(ctx, consts.StatusBadRequest, err.Error())
		return
	}

	// Read merged config from registry for persistence.
	var merged llm.ProviderConfig
	for _, pc := range h.registry.ListProviders() {
		if pc.ID == providerID {
			merged = pc
			break
		}
	}

	if err := h.repo.Upsert(c, llm.ProviderConfigToDomain(merged)); err != nil {
		writeError(ctx, consts.StatusInternalServerError, "failed to persist provider update")
		return
	}

	ctx.JSON(consts.StatusOK, toProviderResponse(merged))
}

// DeleteProvider removes an LLM provider from the registry and the database.
func (h *LLMProviderHandler) DeleteProvider(c context.Context, ctx *app.RequestContext) {
	providerID := ctx.Param("providerID")

	if err := h.registry.RemoveProvider(providerID); err != nil {
		writeError(ctx, consts.StatusBadRequest, err.Error())
		return
	}

	// Best-effort delete from DB; registry is already updated.
	_ = h.repo.Delete(c, providerID)

	ctx.JSON(consts.StatusNoContent, nil)
}

// maskAPIKey masks an API key for display, showing only the first 3 and last 4 characters.
func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:3] + "..." + key[len(key)-4:]
}

func toProviderResponse(cfg llm.ProviderConfig) providerResponse {
	return providerResponse{
		ID:         cfg.ID,
		Provider:   cfg.Provider,
		Model:      cfg.Model,
		BaseURL:    cfg.BaseURL,
		APIKey:     maskAPIKey(cfg.APIKey),
		TimeoutSec: cfg.TimeoutSec,
		Priority:   cfg.Priority,
		Enabled:    cfg.Enabled,
	}
}
