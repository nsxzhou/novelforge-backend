package middleware

import (
	"context"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

const (
	allowMethodsValue  = "GET,POST,PUT,DELETE,OPTIONS"
	allowHeadersValue  = "Content-Type,X-User-ID,X-Request-ID"
	exposeHeadersValue = "X-Request-ID"

	headerOrigin        = "Origin"
	headerVary          = "Vary"
	headerAllowOrigin   = "Access-Control-Allow-Origin"
	headerAllowMethods  = "Access-Control-Allow-Methods"
	headerAllowHeaders  = "Access-Control-Allow-Headers"
	headerExposeHeaders = "Access-Control-Expose-Headers"
)

type corsErrorResponse struct {
	Error string `json:"error"`
}

// CORS applies a strict origin whitelist to API requests.
func CORS(allowedOrigins []string) app.HandlerFunc {
	allowList := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		trimmed := strings.TrimSpace(origin)
		if trimmed == "" {
			continue
		}
		allowList[trimmed] = struct{}{}
	}

	return func(c context.Context, ctx *app.RequestContext) {
		origin := strings.TrimSpace(string(ctx.GetHeader(headerOrigin)))
		if origin == "" {
			ctx.Next(c)
			return
		}

		if _, allowed := allowList[origin]; !allowed {
			if string(ctx.Method()) == consts.MethodOptions {
				ctx.AbortWithStatusJSON(consts.StatusForbidden, corsErrorResponse{Error: "cors origin not allowed"})
				return
			}
			ctx.Next(c)
			return
		}

		// 允许来源命中时，统一设置跨域响应头。
		ctx.Header(headerAllowOrigin, origin)
		ctx.Header(headerVary, headerOrigin)
		ctx.Header(headerExposeHeaders, exposeHeadersValue)

		if string(ctx.Method()) == consts.MethodOptions {
			ctx.Header(headerAllowMethods, allowMethodsValue)
			ctx.Header(headerAllowHeaders, allowHeadersValue)
			ctx.AbortWithStatus(consts.StatusNoContent)
			return
		}

		ctx.Next(c)
	}
}
