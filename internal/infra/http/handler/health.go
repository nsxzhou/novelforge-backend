package handler

import (
	"context"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type healthResponse struct {
	Status string `json:"status"`
}

// ReadinessChecker reports whether the service is ready to serve traffic.
type ReadinessChecker interface {
	CheckReadiness(ctx context.Context) error
}

// HealthHandler serves liveness and readiness endpoints.
type HealthHandler struct {
	readiness ReadinessChecker
}

// NewHealthHandler creates a health handler.
func NewHealthHandler(readiness ReadinessChecker) *HealthHandler {
	return &HealthHandler{readiness: readiness}
}

// Healthz reports whether the service process is alive.
func (h *HealthHandler) Healthz(c context.Context, ctx *app.RequestContext) {
	ctx.JSON(consts.StatusOK, healthResponse{Status: "ok"})
}

// Readyz reports whether the service is ready to serve traffic.
func (h *HealthHandler) Readyz(c context.Context, ctx *app.RequestContext) {
	if h.readiness != nil {
		readyCtx, cancel := context.WithTimeout(c, 2*time.Second)
		defer cancel()
		if err := h.readiness.CheckReadiness(readyCtx); err != nil {
			writeError(ctx, consts.StatusServiceUnavailable, "service not ready")
			return
		}
	}

	ctx.JSON(consts.StatusOK, healthResponse{Status: "ready"})
}
