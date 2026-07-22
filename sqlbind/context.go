//go:build !tinygo

package sqlbind

import (
	"context"
	"database/sql"
	"errors"
)

// ErrNoSQLExecutor reports that a Context does not contain a database
// executor. It is returned instead of panicking so generated Context wrappers
// remain ordinary error-returning APIs.
var ErrNoSQLExecutor = errors.New("sqlbind: no SQL executor in context")

// SQLExecutor is implemented by *sql.DB, *sql.Conn, and *sql.Tx.
type SQLExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

type sqlExecutorContextKey struct{}

// WithSQLExecutor returns a child Context containing a database executor.
// Framework transaction middleware can store a *sql.Tx for generated
// <Component>Context wrappers to resolve.
func WithSQLExecutor(ctx context.Context, executor SQLExecutor) context.Context {
	return context.WithValue(ctx, sqlExecutorContextKey{}, executor)
}

// SQLExecutorFromContext returns the executor installed by WithSQLExecutor.
func SQLExecutorFromContext(ctx context.Context) (SQLExecutor, error) {
	executor, ok := ctx.Value(sqlExecutorContextKey{}).(SQLExecutor)
	if !ok || executor == nil {
		return nil, ErrNoSQLExecutor
	}
	return executor, nil
}
