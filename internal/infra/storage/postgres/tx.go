package postgres

import (
	"context"
	"database/sql"
	"fmt"
)

type txContextKey struct{}

type sqlExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func withTx(ctx context.Context, tx *sql.Tx) context.Context {
	return context.WithValue(ctx, txContextKey{}, tx)
}

func executorFromContext(ctx context.Context, fallback sqlExecutor) sqlExecutor {
	if fallback == nil {
		return nil
	}
	if tx, ok := ctx.Value(txContextKey{}).(*sql.Tx); ok && tx != nil {
		return tx
	}
	return fallback
}

// TxRunner 在 PostgreSQL 连接上执行事务边界。
type TxRunner struct {
	db *sql.DB
}

// NewTxRunner creates a PostgreSQL-backed transaction runner.
func NewTxRunner(db *sql.DB) *TxRunner {
	return &TxRunner{db: db}
}

// InTx executes fn in a single database transaction.
func (r *TxRunner) InTx(ctx context.Context, fn func(context.Context) error) error {
	if fn == nil {
		return nil
	}
	if r == nil || r.db == nil {
		return fn(ctx)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	if err := fn(withTx(ctx, tx)); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return fmt.Errorf("rollback transaction: %w; original error: %v", rollbackErr, err)
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}
