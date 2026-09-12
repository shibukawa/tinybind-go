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

type sliceRows struct {
	columns []string
	rows    [][]any
	index   int
}

func (r *sliceRows) Next() bool {
	if r.index >= len(r.rows) {
		return false
	}
	r.index++
	return true
}

func (r *sliceRows) Scan(dest ...any) error {
	if r.index == 0 || r.index > len(r.rows) || len(dest) != len(r.rows[r.index-1]) {
		return errors.New("invalid row scan")
	}
	for i, value := range r.rows[r.index-1] {
		cell, ok := dest[i].(*any)
		if !ok {
			return errors.New("destination is not *any")
		}
		*cell = value
	}
	return nil
}

func (*sliceRows) Err() error                   { return nil }
func (*sliceRows) Close() error                 { return nil }
func (r *sliceRows) Columns() ([]string, error) { return r.columns, nil }

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

func TestForEachReadsCustomRows(t *testing.T) {
	rows := &sliceRows{
		columns: []string{"id", "name"},
		rows:    [][]any{{int64(7), []byte("Ada")}},
	}
	count := 0
	err := sqlbind.ForEach(rows, func(row sqlbind.Row) error {
		count++
		if row["id"] != int64(7) {
			t.Errorf("id = %#v, want int64(7)", row["id"])
		}
		name, ok := row["name"].([]byte)
		if !ok || string(name) != "Ada" {
			t.Errorf("name = %#v, want []byte(Ada)", row["name"])
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("callback ran %d times, want 1", count)
	}
	if err := sqlbind.ForEach(nil, func(sqlbind.Row) error { return nil }); err == nil {
		t.Fatal("expected an error for nil Rows")
	}
}
