package sqlbind_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"net/url"
	"testing"

	"github.com/shibukawa/tinybind-go/sqlbind"
)

// The url boundary is tested through a real database/sql round trip rather than
// by calling Scan directly. The defect it covers was that database/sql refuses
// a url.URL at both ends, and only the driver path exercises that refusal.

type fakeDriver struct{ recorder *fakeRecorder }

type fakeRecorder struct {
	args    []driver.Value
	columns []string
	row     []driver.Value
}

func (d fakeDriver) Open(string) (driver.Conn, error) { return fakeConn{d.recorder}, nil }

type fakeConn struct{ recorder *fakeRecorder }

func (c fakeConn) Prepare(string) (driver.Stmt, error) { return fakeStmt{c.recorder}, nil }
func (c fakeConn) Close() error                        { return nil }
func (c fakeConn) Begin() (driver.Tx, error)           { return nil, io.EOF }

type fakeStmt struct{ recorder *fakeRecorder }

func (s fakeStmt) Close() error  { return nil }
func (s fakeStmt) NumInput() int { return -1 }
func (s fakeStmt) Exec(args []driver.Value) (driver.Result, error) {
	s.recorder.args = args
	return driver.RowsAffected(1), nil
}
func (s fakeStmt) Query(args []driver.Value) (driver.Rows, error) {
	s.recorder.args = args
	return &fakeRows{recorder: s.recorder}, nil
}

type fakeRows struct {
	recorder *fakeRecorder
	done     bool
}

func (r *fakeRows) Columns() []string { return r.recorder.columns }
func (r *fakeRows) Close() error      { return nil }
func (r *fakeRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	copy(dest, r.recorder.row)
	return nil
}

func openFake(t *testing.T, columns []string, row []driver.Value) (*sql.DB, *fakeRecorder) {
	t.Helper()
	recorder := &fakeRecorder{columns: columns, row: row}
	db := sql.OpenDB(fakeConnector{fakeDriver{recorder}})
	t.Cleanup(func() { db.Close() })
	return db, recorder
}

type fakeConnector struct{ driver fakeDriver }

func (c fakeConnector) Connect(context.Context) (driver.Conn, error) { return c.driver.Open("") }
func (c fakeConnector) Driver() driver.Driver                        { return c.driver }

func TestBuilderBindsURLAsText(t *testing.T) {
	link, err := url.Parse("https://example.com/a?b=1")
	if err != nil {
		t.Fatal(err)
	}
	builder := sqlbind.NewBuilder(sqlbind.Dollar)
	builder.WriteString("SELECT 1 WHERE link = ")
	builder.Arg(*link)
	builder.WriteString(" OR link = ")
	builder.Arg(link)
	builder.WriteString(" OR link = ")
	builder.Arg((*url.URL)(nil))
	statement := builder.Statement()

	want := []any{"https://example.com/a?b=1", "https://example.com/a?b=1", nil}
	if len(statement.Args) != len(want) {
		t.Fatalf("Args = %#v", statement.Args)
	}
	for i, value := range want {
		if statement.Args[i] != value {
			t.Fatalf("Args[%d] = %#v, want %#v", i, statement.Args[i], value)
		}
	}

	// A url.URL reaches the driver only because Arg converted it;
	// driver.DefaultParameterConverter rejects the struct itself.
	db, recorder := openFake(t, []string{"n"}, []driver.Value{int64(1)})
	if _, err := db.ExecContext(context.Background(), statement.SQL, statement.Args...); err != nil {
		t.Fatalf("exec: %v", err)
	}
	if len(recorder.args) != 3 || recorder.args[0] != "https://example.com/a?b=1" || recorder.args[2] != nil {
		t.Fatalf("driver received %#v", recorder.args)
	}
}

func TestScanURLParsesTextColumn(t *testing.T) {
	for _, row := range []driver.Value{"https://example.com/a", []byte("https://example.com/a")} {
		db, _ := openFake(t, []string{"link"}, []driver.Value{row})
		rows, err := db.QueryContext(context.Background(), "SELECT link")
		if err != nil {
			t.Fatal(err)
		}
		if !rows.Next() {
			t.Fatalf("no row: %v", rows.Err())
		}
		var link url.URL
		if err := rows.Scan(sqlbind.ScanURL(&link)); err != nil {
			t.Fatalf("scan %T: %v", row, err)
		}
		rows.Close()
		if link.String() != "https://example.com/a" {
			t.Fatalf("link = %q", link.String())
		}
	}
}

func TestScanURLRejectsNullAndNonText(t *testing.T) {
	for name, row := range map[string]driver.Value{"null": nil, "int": int64(3)} {
		db, _ := openFake(t, []string{"link"}, []driver.Value{row})
		rows, err := db.QueryContext(context.Background(), "SELECT link")
		if err != nil {
			t.Fatal(err)
		}
		if !rows.Next() {
			t.Fatalf("no row: %v", rows.Err())
		}
		var link url.URL
		err = rows.Scan(sqlbind.ScanURL(&link))
		rows.Close()
		if err == nil {
			t.Fatalf("%s: scan succeeded, want error", name)
		}
	}
}

func TestScanOptionalURLKeepsNullNil(t *testing.T) {
	db, _ := openFake(t, []string{"link"}, []driver.Value{nil})
	rows, err := db.QueryContext(context.Background(), "SELECT link")
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() {
		t.Fatalf("no row: %v", rows.Err())
	}
	link := &url.URL{Host: "stale"}
	if err := rows.Scan(sqlbind.ScanOptionalURL(&link)); err != nil {
		t.Fatalf("scan: %v", err)
	}
	rows.Close()
	if link != nil {
		t.Fatalf("link = %#v, want nil", link)
	}
}

func TestScanOptionalURLParsesPresentValue(t *testing.T) {
	db, _ := openFake(t, []string{"link"}, []driver.Value{"https://example.com/b"})
	rows, err := db.QueryContext(context.Background(), "SELECT link")
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() {
		t.Fatalf("no row: %v", rows.Err())
	}
	var link *url.URL
	if err := rows.Scan(sqlbind.ScanOptionalURL(&link)); err != nil {
		t.Fatalf("scan: %v", err)
	}
	rows.Close()
	if link == nil || link.String() != "https://example.com/b" {
		t.Fatalf("link = %#v", link)
	}
}

// TestRawURLIsUnusableWithoutTheAdapter records why the adapter exists: the
// plain field address and the plain struct both fail at the database/sql
// boundary, on every driver.
func TestRawURLIsUnusableWithoutTheAdapter(t *testing.T) {
	if _, err := driver.DefaultParameterConverter.ConvertValue(url.URL{}); err == nil {
		t.Fatal("driver accepted a url.URL parameter; the Arg conversion may be unnecessary now")
	}
	db, _ := openFake(t, []string{"link"}, []driver.Value{"https://example.com/a"})
	rows, err := db.QueryContext(context.Background(), "SELECT link")
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() {
		t.Fatalf("no row: %v", rows.Err())
	}
	var link url.URL
	err = rows.Scan(&link)
	rows.Close()
	if err == nil {
		t.Fatal("database/sql scanned into a url.URL; the scan adapter may be unnecessary now")
	}
}
