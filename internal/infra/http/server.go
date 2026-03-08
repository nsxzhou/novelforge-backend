package http

import (
	"time"

	apiroutes "novelforge/backend/api/http"
	"novelforge/backend/internal/infra/http/middleware"
	"novelforge/backend/pkg/config"

	"github.com/cloudwego/hertz/pkg/app/server"
)

// Dependencies contains route registration dependencies.
type Dependencies = apiroutes.Dependencies

// NewServer creates and configures a Hertz HTTP server.
func NewServer(cfg config.ServerConfig, deps Dependencies) *server.Hertz {
	h := server.Default(
		server.WithHostPorts(cfg.Address()),
		server.WithReadTimeout(time.Duration(cfg.ReadTimeoutSeconds)*time.Second),
		server.WithWriteTimeout(time.Duration(cfg.WriteTimeoutSeconds)*time.Second),
	)
	// 注册全局中间件
	h.Use(middleware.RequestID(), middleware.Recovery())

	apiroutes.RegisterRoutes(h, deps)

	return h
}
