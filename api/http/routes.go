package http

import (
	"novelforge/backend/internal/infra/http/handler"
	assetservice "novelforge/backend/internal/service/asset"
	projectservice "novelforge/backend/internal/service/project"

	"github.com/cloudwego/hertz/pkg/app/server"
)

// Dependencies 包含路由级别的服务。
type Dependencies struct {
	Projects  projectservice.UseCase
	Assets    assetservice.UseCase
	Readiness handler.ReadinessChecker
}

// RegisterRoutes 注册所有 HTTP 路由。
func RegisterRoutes(h *server.Hertz, deps Dependencies) {
	healthHandler := handler.NewHealthHandler(deps.Readiness)
	h.GET("/healthz", healthHandler.Healthz)
	h.GET("/readyz", healthHandler.Readyz)

	projectHandler := handler.NewProjectHandler(deps.Projects)
	assetHandler := handler.NewAssetHandler(deps.Assets)

	v1 := h.Group("/api/v1")
	v1.POST("/projects", projectHandler.Create)
	v1.GET("/projects", projectHandler.List)
	v1.GET("/projects/:projectID", projectHandler.GetByID)
	v1.PUT("/projects/:projectID", projectHandler.Update)
	v1.POST("/projects/:projectID/assets", assetHandler.Create)
	v1.GET("/projects/:projectID/assets", assetHandler.ListByProject)
	v1.GET("/assets/:assetID", assetHandler.GetByID)
	v1.PUT("/assets/:assetID", assetHandler.Update)
	v1.DELETE("/assets/:assetID", assetHandler.Delete)
}
