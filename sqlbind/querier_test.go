package sqlbind_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/shibukawa/tinybind-go/sqlbind"
)

var (
	_ sqlbind.Querier = (*sql.DB)(nil)
	_ sqlbind.Querier = (*sql.Conn)(nil)
	_ sqlbind.Querier = (*sql.Tx)(nil)
	_ sqlbind.Rows    = (*sql.Rows)(nil)
)

type emptyRows struct{}

func (emptyRows) Next() bool                 { return false }
func (emptyRows) Scan(...any) error          { return nil }
func (emptyRows) Err() error                 { return nil }
func (emptyRows) Close() error               { return nil }
func (emptyRows) Columns() ([]string, error) { return nil, nil }

// bothExecutor has the std QueryContext and the driver-agnostic QueryRows, so
// it observes which one Query dispatches to.
type bothExecutor struct{ called *string }

func (e bothExecutor) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	*e.called = "QueryContext"
	return nil, errors.New("unexpected std query")
}

func (e bothExecutor) QueryRows(context.Context, string, ...any) (sqlbind.Rows, error) {
	*e.called = "QueryRows"
	return emptyRows{}, nil
}

func TestQueryPrefersRowsQuerier(t *testing.T) {
	var called string
	rows, err := sqlbind.Query(context.Background(), bothExecutor{called: &called}, "SELECT 1")
	if err != nil {
		t.Fatal(err)
	}
	if called != "QueryRows" {
		t.Fatalf("dispatched to %s, want QueryRows", called)
	}
	if _, ok := rows.(emptyRows); !ok {
		t.Fatalf("rows = %T, want the executor's own implementation", rows)
	}
}

func TestUnimplementedQuerierRejectsStdQuery(t *testing.T) {
	var q sqlbind.UnimplementedQuerier
	if _, err := q.QueryContext(context.Background(), "SELECT 1"); err == nil {
		t.Fatal("expected an error from the stub QueryContext")
	}
}

func TestForEachAcceptsCustomRows(t *testing.T) {
	if err := sqlbind.ForEach(emptyRows{}, func(sqlbind.Row) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := sqlbind.ForEach(nil, func(sqlbind.Row) error { return nil }); err == nil {
		t.Fatal("expected an error for nil Rows")
	}
}
