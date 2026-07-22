//go:build !tinygo

package sqlbind_test

import (
	"context"
	"database/sql"
	"errors"
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
