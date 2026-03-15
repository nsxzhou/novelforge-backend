package postgres

import (
	"context"
	"database/sql"
	"regexp"
	"testing"

	"inkmuse/backend/pkg/config"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestNewProviderRequiresConfiguredEnv(t *testing.T) {
	cfg := config.PostgresConfig{URLEnv: "INKMUSE_DATABASE_URL"}
	t.Setenv(cfg.URLEnv, "")

	provider, err := NewProvider(context.Background(), cfg)
	if err == nil {
		if provider != nil {
			_ = provider.Close()
		}
		t.Fatal("NewProvider() error = nil, want missing env error")
	}
}

func TestNewProviderPingsDatabase(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	previousOpenDB := openDB
	openDB = func(driverName, dsn string) (*sql.DB, error) {
		if driverName != "pgx" {
			t.Fatalf("driverName = %q, want pgx", driverName)
		}
		if dsn != "postgres://example" {
			t.Fatalf("dsn = %q, want postgres://example", dsn)
		}
		return db, nil
	}
	defer func() { openDB = previousOpenDB }()

	mock.ExpectPing()
	cfg := config.PostgresConfig{
		URLEnv:                 "INKMUSE_DATABASE_URL",
		MaxOpenConns:           10,
		MaxIdleConns:           5,
		ConnMaxLifetimeSeconds: 30,
	}
	t.Setenv(cfg.URLEnv, "postgres://example")

	provider, err := NewProvider(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	defer provider.Close()

	if provider.DB() != db {
		t.Fatal("provider.DB() returned unexpected handle")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations not met: %v", err)
	}
}

func TestProviderCheckReadinessUsesPing(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	provider := NewProviderWithDB(db)
	mock.ExpectPing()

	if err := provider.CheckReadiness(context.Background()); err != nil {
		t.Fatalf("CheckReadiness() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations not met: %v", err)
	}
}

func TestMapExecErrorMapsUniqueViolation(t *testing.T) {
	err := mapExecError(testSQLStateError{state: "23505"})
	if err == nil || !regexp.MustCompile(`already exists`).MatchString(err.Error()) {
		t.Fatalf("mapExecError() = %v, want already exists", err)
	}
}

type testSQLStateError struct{ state string }

func (e testSQLStateError) Error() string    { return "sql state error" }
func (e testSQLStateError) SQLState() string { return e.state }
