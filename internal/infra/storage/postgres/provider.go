package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"novelforge/backend/pkg/config"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var openDB = sql.Open

// Provider 管理 PostgreSQL 连通性，用于存储库。
type Provider struct {
	db *sql.DB
}

// OpenStore 打开 PostgreSQL 存储，用于迁移或存储库连接。
func OpenStore(ctx context.Context, cfg config.PostgresConfig) (*Provider, error) {
	return NewProvider(ctx, cfg)
}

// NewProvider 打开并验证 PostgreSQL 连接。
func NewProvider(ctx context.Context, cfg config.PostgresConfig) (*Provider, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate postgres config: %w", err)
	}

	databaseURL, exists := os.LookupEnv(cfg.URLEnv)
	if !exists || databaseURL == "" {
		return nil, fmt.Errorf("required environment variable %q is not set", cfg.URLEnv)
	}

	db, err := openDB("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres connection: %w", err)
	}

	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetimeSeconds > 0 {
		db.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetimeSeconds) * time.Second)
	}

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres connection: %w", err)
	}

	return &Provider{db: db}, nil
}

// NewProviderWithDB wraps an existing SQL database handle.
func NewProviderWithDB(db *sql.DB) *Provider {
	return &Provider{db: db}
}

// DB returns the underlying SQL database handle.
func (p *Provider) DB() *sql.DB {
	if p == nil {
		return nil
	}
	return p.db
}

// CheckReadiness reports whether PostgreSQL is reachable.
func (p *Provider) CheckReadiness(ctx context.Context) error {
	if p == nil || p.db == nil {
		return fmt.Errorf("postgres provider is not initialized")
	}
	return p.db.PingContext(ctx)
}

// Close releases PostgreSQL resources.
func (p *Provider) Close() error {
	if p == nil || p.db == nil {
		return nil
	}
	return p.db.Close()
}
