package database

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type transactionContextKey struct{}

func WithTransaction(ctx context.Context, tx DBTX) context.Context {
	return context.WithValue(ctx, transactionContextKey{}, tx)
}

func Executor(ctx context.Context, fallback DBTX) DBTX {
	if tx, ok := ctx.Value(transactionContextKey{}).(DBTX); ok {
		return tx
	}
	return fallback
}
