package sqlbind

import (
	"context"
	"errors"
	"fmt"
)

// ErrNoSQLExecutor reports that a Context does not contain a database
// executor. It is returned instead of panicking so generated Context wrappers
// remain ordinary error-returning APIs.
var ErrNoSQLExecutor = errors.New("sqlbind: no SQL executor in context")

// ErrReadOnlyExecutor reports that a write statement resolved an executor the
// caller declared read-only. It is returned before the statement is built, so a
// misrouted write fails deterministically instead of depending on whether the
// connection happens to reach a read replica.
var ErrReadOnlyExecutor = errors.New("sqlbind: SQL executor in context is read-only")

// SQLExecutor is implemented by *sql.DB, *sql.Conn, and *sql.Tx. It combines
// the two minimal interfaces generated code takes, so one Context value serves
// both mutating and row-returning components.
type SQLExecutor interface {
	Execer
	Querier
}

type sqlExecutorContextKey struct{}

// sqlExecutorEntry is the executor together with the access mode its caller
// declared. The mode cannot live in the Go type: *sql.Tx satisfies the same
// interface whether or not it was begun read-only, and sql.TxOptions.ReadOnly
// is invisible to the type system.
type sqlExecutorEntry struct {
	executor SQLExecutor
	readOnly bool
}

// ExecutorOption configures how a stored executor may be used.
type ExecutorOption func(*sqlExecutorEntry)

// AsReadOnly marks the stored executor as read-only, so a generated write
// statement resolving it fails with ErrReadOnlyExecutor. Use it for a handle
// pointing at a read replica, or for a transaction begun with
// sql.TxOptions{ReadOnly: true}.
func AsReadOnly() ExecutorOption {
	return func(entry *sqlExecutorEntry) { entry.readOnly = true }
}

// WithSQLExecutor returns a child Context containing a database executor.
// Framework transaction middleware can store a *sql.Tx for generated
// <Component>Context wrappers to resolve.
func WithSQLExecutor(ctx context.Context, executor SQLExecutor, options ...ExecutorOption) context.Context {
	entry := sqlExecutorEntry{executor: executor}
	for _, option := range options {
		option(&entry)
	}
	return context.WithValue(ctx, sqlExecutorContextKey{}, entry)
}

// SQLExecutorFromContext returns the executor installed by WithSQLExecutor. It
// is the resolver generated read statements use, and it accepts a read-only
// executor.
func SQLExecutorFromContext(ctx context.Context) (SQLExecutor, error) {
	entry, ok := ctx.Value(sqlExecutorContextKey{}).(sqlExecutorEntry)
	if !ok || entry.executor == nil {
		return nil, ErrNoSQLExecutor
	}
	return entry.executor, nil
}

// WriteExecutorFromContext is the resolver generated write statements use. It
// rejects an executor stored with AsReadOnly, naming the statement so the error
// identifies which one was misrouted.
func WriteExecutorFromContext(ctx context.Context, statement string) (SQLExecutor, error) {
	entry, ok := ctx.Value(sqlExecutorContextKey{}).(sqlExecutorEntry)
	if !ok || entry.executor == nil {
		return nil, ErrNoSQLExecutor
	}
	if entry.readOnly {
		return nil, fmt.Errorf("%s is a write statement: %w", statement, ErrReadOnlyExecutor)
	}
	return entry.executor, nil
}
