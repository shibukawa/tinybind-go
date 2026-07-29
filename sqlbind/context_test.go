//go:build !tinygo

package sqlbind_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/sqlbind"
)

type contextExecutor struct{ id int }

var _ sqlbind.SQLExecutor = (*sql.DB)(nil)
var _ sqlbind.SQLExecutor = (*sql.Conn)(nil)
var _ sqlbind.SQLExecutor = (*sql.Tx)(nil)

func (*contextExecutor) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	return nil, nil
}
func (*contextExecutor) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	return nil, nil
}

func TestSQLExecutorContext(t *testing.T) {
	if _, err := sqlbind.SQLExecutorFromContext(context.Background()); !errors.Is(err, sqlbind.ErrNoSQLExecutor) {
		t.Fatalf("missing executor error = %v", err)
	}

	first := &contextExecutor{id: 1}
	ctx := sqlbind.WithSQLExecutor(context.Background(), first)
	got, err := sqlbind.SQLExecutorFromContext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got != first {
		t.Fatalf("executor = %#v", got)
	}

	second := &contextExecutor{id: 2}
	got, err = sqlbind.SQLExecutorFromContext(sqlbind.WithSQLExecutor(ctx, second))
	if err != nil {
		t.Fatal(err)
	}
	if got != second {
		t.Fatalf("replacement executor = %#v", got)
	}
}

func TestWriteExecutorFromContext(t *testing.T) {
	executor := &contextExecutor{id: 1}

	if _, err := sqlbind.WriteExecutorFromContext(context.Background(), "DeleteUser"); !errors.Is(err, sqlbind.ErrNoSQLExecutor) {
		t.Fatalf("missing executor error = %v", err)
	}

	// An executor stored without AsReadOnly serves both resolvers.
	writable := sqlbind.WithSQLExecutor(context.Background(), executor)
	got, err := sqlbind.WriteExecutorFromContext(writable, "DeleteUser")
	if err != nil {
		t.Fatal(err)
	}
	if got != executor {
		t.Fatalf("write executor = %#v", got)
	}

	readOnly := sqlbind.WithSQLExecutor(context.Background(), executor, sqlbind.AsReadOnly())
	_, err = sqlbind.WriteExecutorFromContext(readOnly, "DeleteUser")
	if !errors.Is(err, sqlbind.ErrReadOnlyExecutor) {
		t.Fatalf("read-only error = %v", err)
	}
	if !strings.Contains(err.Error(), "DeleteUser") {
		t.Fatalf("error does not name the statement: %v", err)
	}

	// A read statement resolves the same executor without error.
	got, err = sqlbind.SQLExecutorFromContext(readOnly)
	if err != nil {
		t.Fatal(err)
	}
	if got != executor {
		t.Fatalf("read executor = %#v", got)
	}

	// Re-storing without the option clears the mark.
	if _, err := sqlbind.WriteExecutorFromContext(sqlbind.WithSQLExecutor(readOnly, executor), "DeleteUser"); err != nil {
		t.Fatalf("re-stored executor = %v", err)
	}
}
