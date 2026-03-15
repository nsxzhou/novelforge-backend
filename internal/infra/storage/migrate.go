package storage

import (
	"context"
	"fmt"

	"inkmuse/backend/internal/infra/storage/postgres"
	"inkmuse/backend/pkg/config"
)

var newPostgresProviderForMigrations = postgres.NewProvider
var runPostgresMigrations = postgres.RunMigrations

// RunMigrations 应用 schema 迁移，源自配置的存储提供程序。
func RunMigrations(ctx context.Context, cfg config.StorageConfig) error {
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validate storage config: %w", err)
	}

	switch cfg.Provider {
	case config.StorageProviderMemory:
		return nil
	case config.StorageProviderPostgres:
		provider, err := newPostgresProviderForMigrations(ctx, cfg.Postgres)
		if err != nil {
			return fmt.Errorf("open postgres store: %w", err)
		}
		defer provider.Close()

		if err := runPostgresMigrations(ctx, provider.DB()); err != nil {
			return fmt.Errorf("run postgres migrations: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported storage provider %q", cfg.Provider)
	}
}
