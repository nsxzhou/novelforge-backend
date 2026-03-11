package postgres

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestTxRunnerInTxCommit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	runner := NewTxRunner(db)
	mock.ExpectBegin()
	mock.ExpectCommit()

	called := false
	err = runner.InTx(context.Background(), func(_ context.Context) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("InTx() error = %v", err)
	}
	if !called {
		t.Fatal("InTx() did not execute callback")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations not met: %v", err)
	}
}

func TestTxRunnerInTxRollbackOnError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	runner := NewTxRunner(db)
	mock.ExpectBegin()
	mock.ExpectRollback()

	wantErr := sql.ErrTxDone
	err = runner.InTx(context.Background(), func(_ context.Context) error {
		return wantErr
	})
	if err != wantErr {
		t.Fatalf("InTx() error = %v, want %v", err, wantErr)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations not met: %v", err)
	}
}
