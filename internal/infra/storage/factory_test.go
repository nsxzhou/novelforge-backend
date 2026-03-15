package storage

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"inkmuse/backend/internal/infra/storage/postgres"
	"inkmuse/backend/pkg/config"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestNewRepositoriesMemoryHasNoopReadiness(t *testing.T) {
	repos, err := NewRepositories(config.StorageConfig{Provider: config.StorageProviderMemory})
	if err != nil {
		t.Fatalf("NewRepositories() error = %v", err)
	}
	defer repos.Close()

	if repos.Projects == nil || repos.Assets == nil || repos.Chapters == nil || repos.Conversations == nil || repos.GenerationRecords == nil || repos.MetricEvents == nil {
		t.Fatal("NewRepositories() returned nil repository dependency")
	}
	if err := repos.CheckReadiness(context.Background()); err != nil {
		t.Fatalf("CheckReadiness() error = %v", err)
	}
}

func TestNewRepositoriesPostgresWiresProvider(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	previousFactory := newPostgresProvider
	newPostgresProvider = func(_ context.Context, _ config.PostgresConfig) (*postgres.Provider, error) {
		return postgres.NewProviderWithDB(db), nil
	}
	defer func() { newPostgresProvider = previousFactory }()

	repos, err := NewRepositories(config.StorageConfig{Provider: config.StorageProviderPostgres, Postgres: config.PostgresConfig{URLEnv: "INKMUSE_DATABASE_URL"}})
	if err != nil {
		t.Fatalf("NewRepositories() error = %v", err)
	}
	defer repos.Close()

	mock.ExpectPing()
	if err := repos.CheckReadiness(context.Background()); err != nil {
		t.Fatalf("CheckReadiness() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations not met: %v", err)
	}
}

func TestNewRepositoriesPostgresProviderError(t *testing.T) {
	previousFactory := newPostgresProvider
	newPostgresProvider = func(_ context.Context, _ config.PostgresConfig) (*postgres.Provider, error) {
		return nil, errors.New("boom")
	}
	defer func() { newPostgresProvider = previousFactory }()

	_, err := NewRepositories(config.StorageConfig{Provider: config.StorageProviderPostgres, Postgres: config.PostgresConfig{URLEnv: "INKMUSE_DATABASE_URL"}})
	if err == nil {
		t.Fatal("NewRepositories() error = nil, want provider error")
	}
}

func TestRunMigrationsPostgresUsesProviderAndRunner(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	previousFactory := newPostgresProviderForMigrations
	previousRunner := runPostgresMigrations
	newPostgresProviderForMigrations = func(_ context.Context, _ config.PostgresConfig) (*postgres.Provider, error) {
		return postgres.NewProviderWithDB(db), nil
	}
	called := false
	runPostgresMigrations = func(_ context.Context, gotDB *sql.DB) error {
		called = true
		if gotDB != db {
			t.Fatal("RunMigrations() passed unexpected db")
		}
		return nil
	}
	defer func() {
		newPostgresProviderForMigrations = previousFactory
		runPostgresMigrations = previousRunner
	}()

	if err := RunMigrations(context.Background(), config.StorageConfig{Provider: config.StorageProviderPostgres, Postgres: config.PostgresConfig{URLEnv: "INKMUSE_DATABASE_URL"}}); err != nil {
		t.Fatalf("RunMigrations() error = %v", err)
	}
	if !called {
		t.Fatal("RunMigrations() did not call postgres migration runner")
	}
}
