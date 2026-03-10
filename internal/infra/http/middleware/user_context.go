package middleware

import (
	"context"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/google/uuid"
)

const (
	// UserIDHeader 用于从请求头透传当前用户标识。
	UserIDHeader = "X-User-ID"
	// UserIDContextKey 是请求上下文中的用户标识键。
	UserIDContextKey = "user_id"
)

// UserContext 将有效的用户 ID 写入请求上下文，供业务 handler 使用。
func UserContext() app.HandlerFunc {
	return func(c context.Context, ctx *app.RequestContext) {
		userID := strings.TrimSpace(string(ctx.GetHeader(UserIDHeader)))
		if userID != "" {
			if _, err := uuid.Parse(userID); err == nil {
				ctx.Set(UserIDContextKey, userID)
			}
		}
		ctx.Next(c)
	}
}
