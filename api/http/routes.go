package http

import (
	"novelforge/backend/internal/infra/http/handler"
	assetservice "novelforge/backend/internal/service/asset"
	chapterservice "novelforge/backend/internal/service/chapter"
	conversationservice "novelforge/backend/internal/service/conversation"
	projectservice "novelforge/backend/internal/service/project"

	"github.com/cloudwego/hertz/pkg/app/server"
)

// Dependencies 包含路由级别的服务。
type Dependencies struct {
	Projects      projectservice.UseCase
	Assets        assetservice.UseCase
	Chapters      chapterservice.UseCase
	Conversations conversationservice.UseCase
	Readiness     handler.ReadinessChecker
}

// RegisterRoutes 注册所有 HTTP 路由。
func RegisterRoutes(h *server.Hertz, deps Dependencies) {
	healthHandler := handler.NewHealthHandler(deps.Readiness)
	h.GET("/healthz", healthHandler.Healthz)
	h.GET("/readyz", healthHandler.Readyz)

	projectHandler := handler.NewProjectHandler(deps.Projects)
	assetHandler := handler.NewAssetHandler(deps.Assets)
	chapterHandler := handler.NewChapterHandler(deps.Chapters)
	conversationHandler := handler.NewConversationHandler(deps.Conversations)

	v1 := h.Group("/api/v1")
	v1.POST("/projects", projectHandler.Create)
	v1.GET("/projects", projectHandler.List)
	v1.GET("/projects/:projectID", projectHandler.GetByID)
	v1.PUT("/projects/:projectID", projectHandler.Update)
	v1.POST("/projects/:projectID/assets", assetHandler.Create)
	v1.GET("/projects/:projectID/assets", assetHandler.ListByProject)
	v1.POST("/projects/:projectID/chapters", chapterHandler.Create)
	v1.GET("/projects/:projectID/chapters", chapterHandler.ListByProject)
	v1.POST("/projects/:projectID/conversations", conversationHandler.Start)
	v1.GET("/projects/:projectID/conversations", conversationHandler.List)
	v1.GET("/assets/:assetID", assetHandler.GetByID)
	v1.PUT("/assets/:assetID", assetHandler.Update)
	v1.DELETE("/assets/:assetID", assetHandler.Delete)
	v1.GET("/chapters/:chapterID", chapterHandler.GetByID)
	v1.POST("/chapters/:chapterID/confirm", chapterHandler.Confirm)
	v1.POST("/chapters/:chapterID/continue", chapterHandler.Continue)
	v1.POST("/chapters/:chapterID/rewrite", chapterHandler.Rewrite)
	v1.GET("/conversations/:conversationID", conversationHandler.GetByID)
	v1.POST("/conversations/:conversationID/messages", conversationHandler.Reply)
	v1.POST("/conversations/:conversationID/confirm", conversationHandler.Confirm)
}
