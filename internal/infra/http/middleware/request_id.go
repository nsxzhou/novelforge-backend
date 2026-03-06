package middleware

import (
	"context"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/google/uuid"
)

const (
	// RequestIDHeader is the HTTP header key used for request id propagation.
	RequestIDHeader = "X-Request-ID"
	// RequestIDContextKey is the request context key where request id is stored.
	RequestIDContextKey = "request_id"
)

// RequestID ensures every request has a request id.
func RequestID() app.HandlerFunc {
	return func(c context.Context, ctx *app.RequestContext) {
		requestID := strings.TrimSpace(string(ctx.GetHeader(RequestIDHeader)))
		if requestID == "" {
			requestID = uuid.NewString()
		}

		ctx.Set(RequestIDContextKey, requestID)
		ctx.Header(RequestIDHeader, requestID)
		ctx.Next(c)
	}
}
