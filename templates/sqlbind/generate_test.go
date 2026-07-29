package sqlbind_test

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/sqlbind"
)

func TestGenerateAndRunBuilder(t *testing.T) {
	source := []byte(`package queries
type User { id: int, name: string }
export statement Find(id: int, names: string[], active: bool): sql.one<User> {
SELECT id, name FROM users WHERE id = {id} AND name IN ({names}) {if active}AND active = {true}{/if}
}`)
	generated, err := sqlbind.Generate("users.tb.sql", source, sqlbind.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	runtimeTest := []byte(`package queries
import "testing"
func TestBuilder(t *testing.T) {
	statement, err := BuildFind(7, []string{"a", "b"}, true)
	if err != nil { t.Fatal(err) }
	if statement.SQL != "\nSELECT id, name FROM users WHERE id = $1 AND name IN ($2, $3) AND active = $4\n" { t.Fatalf("SQL = %q", statement.SQL) }
	if len(statement.Args) != 4 || statement.Args[0] != 7 || statement.Args[3] != true { t.Fatalf("Args = %#v", statement.Args) }
}`)
	runGenerated(t, generated, runtimeTest)
}

func TestGenerateExecMutationAndQuestionPlaceholders(t *testing.T) {
	source := []byte(`package queries
export statement Rename(id: int, name: string, enabled: bool): sql.exec {
UPDATE users SET name = {name} WHERE id = {id} {if enabled}AND enabled{/if}
}`)
	generated, err := sqlbind.Generate("users.tb.sql", source, sqlbind.GenerateOptions{PlaceholderStyle: "question"})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(generated, []byte("_tinybindsql.NewBuilder(_tinybindsql.Question)")) {
		t.Fatalf("question placeholder style not selected:\n%s", generated)
	}
	runGenerated(t, generated, nil)
}

func TestPredicateAndRelationCompositionSharePlaceholderOrder(t *testing.T) {
	source := []byte(`package queries
type User { id: int, name: string }
statement MinimumID(id: int): sql.predicate {id >= {id}}
statement ActiveUsers(minimum: int): sql.relation<User> {SELECT id, name FROM users WHERE {MinimumID(minimum)}}
export statement Find(name: string, minimum: int): sql.many<User> {
SELECT active_users.id, active_users.name FROM subquery ActiveUsers(minimum) AS active_users WHERE active_users.name = {name}
}`)
	generated, err := sqlbind.Generate("relations.tb.sql", source, sqlbind.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(generated, []byte("iter.Seq2[User, error]")) {
		t.Fatalf("sql.many API is not an iterator:\n%s", generated)
	}
	runtimeTest := []byte(`package queries
import (
    "context"
    "database/sql"
    "errors"
    "strings"
    "testing"
)
type failingQuerier struct{}
func (failingQuerier) QueryContext(context.Context, string, ...any) (*sql.Rows, error) { return nil, errors.New("query failed") }
func TestRelations(t *testing.T) {
	statement, err := BuildFind("Ada", 10)
	if err != nil { t.Fatal(err) }
	if !strings.Contains(statement.SQL, "FROM (SELECT id, name FROM users WHERE id >= $1) AS active_users") { t.Fatalf("SQL = %q", statement.SQL) }
	if !strings.Contains(statement.SQL, "active_users.name = $2") { t.Fatalf("SQL = %q", statement.SQL) }
	if len(statement.Args) != 2 || statement.Args[0] != 10 || statement.Args[1] != "Ada" { t.Fatalf("Args = %#v", statement.Args) }
	count := 0
	for _, err := range Find(context.Background(), failingQuerier{}, "Ada", 10) { count++; if err == nil || err.Error() != "query failed" { t.Fatalf("err = %v", err) } }
	if count != 1 { t.Fatalf("error yields = %d", count) }
}`)
	runGenerated(t, generated, runtimeTest)
}

func TestGenerateContextAPI(t *testing.T) {
	source := []byte(`package queries
type User { id: int, name: string }
export statement GetUser(id: int): sql.one<User> {SELECT id, name FROM users WHERE id = {id}}
export statement MaybeUser(id: int): sql.optional<User> {SELECT id, name FROM users WHERE id = {id}}
export statement ListUsers(): sql.many<User> {SELECT id, name FROM users}
export statement DeleteUser(id: int): sql.exec {DELETE FROM users WHERE id = {id}}`)
	generated, err := sqlbind.Generate("context.tb.sql", source, sqlbind.GenerateOptions{ContextAPI: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, signature := range []string{"func GetUser(ctx context.Context, db _tinybindsql.Querier", "func GetUserContext(ctx context.Context", "func MaybeUserContext(ctx context.Context", "func ListUsersContext(ctx context.Context", "func DeleteUserContext(ctx context.Context"} {
		if !bytes.Contains(generated, []byte(signature)) {
			t.Fatalf("generated output lacks %q:\n%s", signature, generated)
		}
	}
	runtimeTest := []byte(`package queries
import (
    "context"
    "database/sql"
    "database/sql/driver"
    "errors"
    "io"
    "sync/atomic"
    "testing"
    rootsql "github.com/shibukawa/tinybind-go/sqlbind"
)
var errQuery = errors.New("query failed")
var errExec = errors.New("exec failed")
type failingExecutor struct{}
func (failingExecutor) QueryContext(context.Context, string, ...any) (*sql.Rows, error) { return nil, errQuery }
func (failingExecutor) ExecContext(context.Context, string, ...any) (sql.Result, error) { return nil, errExec }
var rowsClosed atomic.Bool
type rowsDriver struct{}
type rowsConn struct{}
type oneRow struct{ sent bool }
func (rowsDriver) Open(string) (driver.Conn, error) { return rowsConn{}, nil }
func (rowsConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not implemented") }
func (rowsConn) Close() error { return nil }
func (rowsConn) Begin() (driver.Tx, error) { return nil, errors.New("not implemented") }
func (rowsConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) { return &oneRow{}, nil }
func (*oneRow) Columns() []string { return []string{"id", "name"} }
func (*oneRow) Close() error { rowsClosed.Store(true); return nil }
func (r *oneRow) Next(values []driver.Value) error { if r.sent { return io.EOF }; r.sent = true; values[0] = int64(1); values[1] = "Ada"; return nil }
func TestContextAPIs(t *testing.T) {
    if _, err := GetUserContext(context.Background(), 1); !errors.Is(err, rootsql.ErrNoSQLExecutor) { t.Fatalf("missing executor error = %v", err) }
    if result, err := MaybeUserContext(context.Background(), 1); result != nil || !errors.Is(err, rootsql.ErrNoSQLExecutor) { t.Fatalf("missing optional result=%v error=%v", result, err) }
    missingSeq := ListUsersContext(context.Background())
    missingCount := 0
    for _, err := range missingSeq { missingCount++; if !errors.Is(err, rootsql.ErrNoSQLExecutor) { t.Fatalf("missing many error = %v", err) } }
    if missingCount != 1 { t.Fatalf("missing resolver yields = %d", missingCount) }
    ctx := rootsql.WithSQLExecutor(context.Background(), failingExecutor{})
    if _, err := GetUserContext(ctx, 1); !errors.Is(err, errQuery) { t.Fatalf("query error = %v", err) }
    if _, err := MaybeUserContext(ctx, 1); !errors.Is(err, errQuery) { t.Fatalf("optional query error = %v", err) }
    if _, err := DeleteUserContext(ctx, 1); !errors.Is(err, errExec) { t.Fatalf("exec error = %v", err) }
    count := 0
    for _, err := range ListUsersContext(ctx) { count++; if !errors.Is(err, errQuery) { t.Fatalf("many error = %v", err) } }
    if count != 1 { t.Fatalf("many error yields = %d", count) }
    sql.Register("tinybind-context-rows", rowsDriver{})
    db, err := sql.Open("tinybind-context-rows", "")
    if err != nil { t.Fatal(err) }
    defer db.Close()
    rowsClosed.Store(false)
    for user, err := range ListUsersContext(rootsql.WithSQLExecutor(context.Background(), db)) { if err != nil { t.Fatal(err) }; if user.Id != 1 { t.Fatalf("user = %#v", user) }; break }
    if !rowsClosed.Load() { t.Fatal("rows were not closed after early iterator stop") }
}`)
	runGenerated(t, generated, runtimeTest)
}

func TestGenerateCustomContextResolver(t *testing.T) {
	source := []byte(`package queries
type User { id: int }
export statement GetUser(id: int): sql.one<User> {SELECT id FROM users WHERE id = {id}}`)
	generated, err := sqlbind.Generate("custom.tb.sql", source, sqlbind.GenerateOptions{ExecutorResolver: &sqlbind.ExecutorResolver{PackagePath: "example.com/web/dbctx", Name: "Executor"}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(generated, []byte(`_tinybindresolver "example.com/web/dbctx"`)) || !bytes.Contains(generated, []byte(`_tinybindresolver.Executor(ctx)`)) {
		t.Fatalf("custom resolver missing:\n%s", generated)
	}
	// The runtime package is always imported for Statement and Builder; only
	// the standard Context resolver must be absent.
	if bytes.Contains(generated, []byte(`_tinybindsql.SQLExecutorFromContext`)) {
		t.Fatalf("standard resolver used with custom resolver:\n%s", generated)
	}
	if _, err := sqlbind.Generate("custom.tb.sql", source, sqlbind.GenerateOptions{ExecutorResolver: &sqlbind.ExecutorResolver{Name: "not-exported"}}); err == nil || !strings.Contains(err.Error(), "invalid SQL executor resolver") {
		t.Fatalf("invalid resolver error = %v", err)
	}
}

func TestGenerateDiagnostics(t *testing.T) {
	tests := []struct{ source, want string }{
		{`statement Bad(id: int): sql.exec { DELETE FROM users }`, "require a WHERE"},
		{`statement Bad(id: int): sql.exec { SELECT $1 }`, "manual SQL placeholders"},
		{`statement Bad(value: Missing): sql.exec { SELECT {value} }`, "unknown type Missing"},
		{`statement Bad(values: string[]): sql.exec { SELECT {for value in values}{value}{/for} }`, "general SQL loops"},
		{`type Row { id: int } statement Loop(): sql.relation<Row> {SELECT id FROM subquery Loop() AS loop_rows}`, "recursive SQL composition"},
		{`type Row { id: int } export statement Bad(ok: bool): sql.many<Row> {SELECT id {if ok}, 2{/if} FROM rows}`, "runtime-conditional select columns"},
		{`type Row { id: int, name: string } export statement Bad(): sql.many<Row> {SELECT id FROM rows}`, "has 1 columns"},
		{`type Row { id: int } export statement Bad(): sql.many<Row> {SELECT other FROM rows}`, `column "other" does not match field "id"`},
		{`type Row { id: int } export statement Get(): sql.one<Row> {SELECT id FROM rows} statement GetContext(): sql.exec {SELECT 1}`, "generated Context API conflicts"},
	}
	for _, test := range tests {
		options := sqlbind.GenerateOptions{}
		if strings.Contains(test.want, "Context API conflicts") {
			options.ContextAPI = true
		}
		_, err := sqlbind.Generate("bad.tb.sql", []byte(test.source), options)
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("error = %v, want %q", err, test.want)
		}
	}
}

func TestSQLLiteralsAndCommentsAreLossless(t *testing.T) {
	source := []byte("statement Safe(): sql.exec { SELECT '{not_template}', $$ {also_not} $$/** {still_not} */ -- $1 {comment}\n }")
	if _, err := sqlbind.Generate("safe.tb.sql", source, sqlbind.GenerateOptions{}); err != nil {
		t.Fatal(err)
	}
}

func runGenerated(t *testing.T, generated, runtimeTest []byte) {
	t.Helper()
	dir := t.TempDir()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	goMod := fmt.Sprintf("module fixture\n\ngo 1.26\n\nrequire github.com/shibukawa/tinybind-go v0.0.0\nreplace github.com/shibukawa/tinybind-go => %s\n", root)
	files := map[string][]byte{"go.mod": []byte(goMod), "generated.go": generated}
	if runtimeTest != nil {
		files["generated_test.go"] = runtimeTest
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	command := exec.Command("go", "test", ".")
	command.Dir = dir
	command.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generated code failed: %v\n%s\n%s", err, output, generated)
	}
}

func TestGenerateContextOnlyAPI(t *testing.T) {
	source := []byte(`package queries
type User { id: int, name: string }
export statement FindUser(id: int): sql.one<User> {SELECT id, name FROM users WHERE id = {id}}
export statement MaybeUser(id: int): sql.optional<User> {SELECT id, name FROM users WHERE id = {id}}
export statement ListUsers(): sql.many<User> {SELECT id, name FROM users}
export statement DeleteUser(id: int): sql.exec {DELETE FROM users WHERE id = {id}}`)
	generated, err := sqlbind.Generate("users.pw.sql", source, sqlbind.GenerateOptions{ContextOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"func FindUser(ctx context.Context, id int) (User, error)",
		"func MaybeUser(ctx context.Context, id int) (*User, error)",
		"func ListUsers(ctx context.Context) iter.Seq2[User, error]",
		"func DeleteUser(ctx context.Context, id int) (sql.Result, error)",
		"func BuildFindUser(id int) (_tinybindsql.Statement, error)",
		"func _tinybindExecFindUser(ctx context.Context, db _tinybindsql.Querier",
	}
	for _, signature := range want {
		if !bytes.Contains(generated, []byte(signature)) {
			t.Fatalf("generated output lacks %q:\n%s", signature, generated)
		}
	}
	for _, unwanted := range []string{"func FindUserContext(", "func ListUsersContext(", "func DeleteUserContext("} {
		if bytes.Contains(generated, []byte(unwanted)) {
			t.Fatalf("context-only output still declares %q:\n%s", unwanted, generated)
		}
	}
	for _, exported := range exportedFuncSignatures(t, generated) {
		if strings.Contains(exported, "_tinybindsql.Querier") || strings.Contains(exported, "_tinybindsql.Execer") {
			t.Fatalf("context-only output exports an executor-taking function: %s", exported)
		}
	}
	runtimeTest := []byte(`package queries
import (
    "context"
    "database/sql"
    "errors"
    "testing"
    rootsql "github.com/shibukawa/tinybind-go/sqlbind"
)
var errQuery = errors.New("query failed")
var errExec = errors.New("exec failed")
type failingExecutor struct{}
func (failingExecutor) QueryContext(context.Context, string, ...any) (*sql.Rows, error) { return nil, errQuery }
func (failingExecutor) ExecContext(context.Context, string, ...any) (sql.Result, error) { return nil, errExec }
func TestContextOnlyAPIs(t *testing.T) {
    if _, err := FindUser(context.Background(), 1); !errors.Is(err, rootsql.ErrNoSQLExecutor) { t.Fatalf("missing executor error = %v", err) }
    ctx := rootsql.WithSQLExecutor(context.Background(), failingExecutor{})
    if _, err := FindUser(ctx, 1); !errors.Is(err, errQuery) { t.Fatalf("query error = %v", err) }
    if _, err := MaybeUser(ctx, 1); !errors.Is(err, errQuery) { t.Fatalf("optional query error = %v", err) }
    if _, err := DeleteUser(ctx, 1); !errors.Is(err, errExec) { t.Fatalf("exec error = %v", err) }
    count := 0
    for _, err := range ListUsers(ctx) { count++; if !errors.Is(err, errQuery) { t.Fatalf("many error = %v", err) } }
    if count != 1 { t.Fatalf("many error yields = %d", count) }
    if statement, err := BuildFindUser(3); err != nil || statement.SQL == "" { t.Fatalf("builder = %q, %v", statement.SQL, err) }
}`)
	runGenerated(t, generated, runtimeTest)
}

func TestGenerateContextOnlyKeepsDeclaredNameFree(t *testing.T) {
	source := []byte(`package queries
type Row { id: int }
export statement Get(): sql.one<Row> {SELECT id FROM rows}
statement GetContext(): sql.exec {SELECT 1}`)
	if _, err := sqlbind.Generate("free.pw.sql", source, sqlbind.GenerateOptions{ContextOnly: true}); err != nil {
		t.Fatalf("context-only generation must not reserve GetContext: %v", err)
	}
}

// exportedFuncSignatures returns the source text of every exported top-level
// function declaration in generated Go source.
func exportedFuncSignatures(t *testing.T, generated []byte) []string {
	t.Helper()
	files := token.NewFileSet()
	file, err := parser.ParseFile(files, "generated.go", generated, parser.AllErrors)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Recv != nil || !function.Name.IsExported() {
			continue
		}
		var buffer bytes.Buffer
		if err := format.Node(&buffer, files, &ast.FuncDecl{Name: function.Name, Type: function.Type}); err != nil {
			t.Fatal(err)
		}
		out = append(out, buffer.String())
	}
	return out
}

// A write statement must resolve its executor through the resolver that rejects
// a read-only Context, and must fail before it reaches the database.
func TestGenerateReadOnlyExecutorRejection(t *testing.T) {
	source := []byte(`package queries
type User { id: int, name: string }
export statement GetUser(id: int): sql.one<User> {SELECT id, name FROM users WHERE id = {id}}
export statement ListUsers(): sql.many<User> {SELECT id, name FROM users}
export statement LockUser(id: int): sql.one<User> {SELECT id, name FROM users WHERE id = {id} FOR UPDATE}
export statement PopUser(id: int): sql.one<User> {DELETE FROM users WHERE id = {id} RETURNING id, name}
export statement PurgeUsers(): sql.many<User> {DELETE FROM users WHERE id > 0 RETURNING id, name}
export statement DeleteUser(id: int): sql.exec {DELETE FROM users WHERE id = {id}}`)
	generated, err := sqlbind.Generate("readonly.tb.sql", source, sqlbind.GenerateOptions{ContextAPI: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"LockUser", "PopUser", "PurgeUsers", "DeleteUser"} {
		call := `_tinybindsql.WriteExecutorFromContext(ctx, "` + name + `")`
		if !bytes.Contains(generated, []byte(call)) {
			t.Fatalf("write statement %s lacks %q:\n%s", name, call, generated)
		}
	}
	if bytes.Count(generated, []byte("_tinybindsql.SQLExecutorFromContext(ctx)")) != 2 {
		t.Fatalf("read statements should be the only users of the plain resolver:\n%s", generated)
	}
	runtimeTest := []byte(`package queries
import (
    "context"
    "database/sql"
    "errors"
    "strings"
    "testing"
    rootsql "github.com/shibukawa/tinybind-go/sqlbind"
)
var errQuery = errors.New("query reached the database")
var errExec = errors.New("exec reached the database")
type failingExecutor struct{}
func (failingExecutor) QueryContext(context.Context, string, ...any) (*sql.Rows, error) { return nil, errQuery }
func (failingExecutor) ExecContext(context.Context, string, ...any) (sql.Result, error) { return nil, errExec }
func TestReadOnlyContextRejectsWrites(t *testing.T) {
    ctx := rootsql.WithSQLExecutor(context.Background(), failingExecutor{}, rootsql.AsReadOnly())

    // Writes stop at the resolver, so the database is never reached.
    if _, err := LockUserContext(ctx, 1); !errors.Is(err, rootsql.ErrReadOnlyExecutor) { t.Fatalf("FOR UPDATE error = %v", err) }
    if _, err := PopUserContext(ctx, 1); !errors.Is(err, rootsql.ErrReadOnlyExecutor) { t.Fatalf("DELETE RETURNING error = %v", err) }
    if _, err := DeleteUserContext(ctx, 1); !errors.Is(err, rootsql.ErrReadOnlyExecutor) { t.Fatalf("exec error = %v", err) }
    purged := 0
    for _, err := range PurgeUsersContext(ctx) { purged++; if !errors.Is(err, rootsql.ErrReadOnlyExecutor) { t.Fatalf("many write error = %v", err) } }
    if purged != 1 { t.Fatalf("many write yields = %d", purged) }

    // The error names the statement that was misrouted.
    _, err := PopUserContext(ctx, 1)
    if !strings.Contains(err.Error(), "PopUser") { t.Fatalf("error does not name the statement: %v", err) }

    // Reads run against the same read-only executor.
    if _, err := GetUserContext(ctx, 1); !errors.Is(err, errQuery) { t.Fatalf("read error = %v", err) }
    listed := 0
    for _, err := range ListUsersContext(ctx) { listed++; if !errors.Is(err, errQuery) { t.Fatalf("many read error = %v", err) } }
    if listed != 1 { t.Fatalf("many read yields = %d", listed) }

    // Without AsReadOnly every statement reaches the database as before.
    writable := rootsql.WithSQLExecutor(context.Background(), failingExecutor{})
    if _, err := PopUserContext(writable, 1); !errors.Is(err, errQuery) { t.Fatalf("writable one error = %v", err) }
    if _, err := DeleteUserContext(writable, 1); !errors.Is(err, errExec) { t.Fatalf("writable exec error = %v", err) }
}`)
	runGenerated(t, generated, runtimeTest)
}
