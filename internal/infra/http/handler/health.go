package handler

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type healthResponse struct {
	Status string `json:"status"`
}

// Healthz reports whether the service process is alive.
func Healthz(c context.Context, ctx *app.RequestContext) {
	ctx.JSON(consts.StatusOK, healthResponse{Status: "ok"})
}

// Readyz reports whether the service is ready to serve traffic.
func Readyz(c context.Context, ctx *app.RequestContext) {
	ctx.JSON(consts.StatusOK, healthResponse{Status: "ready"})
}
