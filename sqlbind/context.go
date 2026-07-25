package sqlbind

import (
	"context"
	"errors"
)

// ErrNoSQLExecutor reports that a Context does not contain a database
// executor. It is returned instead of panicking so generated Context wrappers
// remain ordinary error-returning APIs.
var ErrNoSQLExecutor = errors.New("sqlbind: no SQL executor in context")

// SQLExecutor is implemented by *sql.DB, *sql.Conn, and *sql.Tx. It combines
// the two minimal interfaces generated code takes, so one Context value serves
// both mutating and row-returning components.
type SQLExecutor interface {
	Execer
	Querier
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
