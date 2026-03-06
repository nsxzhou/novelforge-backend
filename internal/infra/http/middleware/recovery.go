package middleware

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// Recovery recovers panics and returns an internal server error.
func Recovery() app.HandlerFunc {
	return func(c context.Context, ctx *app.RequestContext) {
		defer func() {
			if r := recover(); r != nil {
				requestID, _ := ctx.Get(RequestIDContextKey)
				hlog.Errorf("panic recovered request_id=%v panic=%v", requestID, r)
				ctx.AbortWithStatusJSON(consts.StatusInternalServerError, map[string]string{
					"error": "internal server error",
				})
			}
		}()

		ctx.Next(c)
	}
}
