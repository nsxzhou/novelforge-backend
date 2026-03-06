package http

import (
	"context"

	"novelforge/backend/internal/infra/http/handler"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// RegisterRoutes registers all HTTP routes.
func RegisterRoutes(h *server.Hertz) {
	h.GET("/healthz", handler.Healthz)
	h.GET("/readyz", handler.Readyz)

	v1 := h.Group("/api/v1")
	v1.GET("/placeholder", func(c context.Context, ctx *app.RequestContext) {
		ctx.JSON(consts.StatusOK, map[string]string{
			"message": "api v1 placeholder",
		})
	})
}
