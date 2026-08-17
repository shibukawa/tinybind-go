package sqlbind_test

import (
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/sqlbind"
)

// decision:declaration-name-policy: the declaration name is the Go function
// name, in both directions. An executable statement therefore takes its Go
// visibility from its own name's case, and export has to agree with it.

func generateSQL(t *testing.T, source string) (string, error) {
	t.Helper()
	out, err := sqlbind.Generate("names.tb.sql", []byte(source), sqlbind.GenerateOptions{Dialect: sqlbind.DialectPostgreSQL})
	return string(out), err
}

func TestPrivateStatementIsCallableUnderItsOwnName(t *testing.T) {
	source := "package store\ntype Row { id: int }\nstatement findUser(id: int): sql.one<Row> {SELECT id FROM users WHERE id = {id}}"
	out, err := generateSQL(t, source)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	// The point of the change: a handler in this package calls findUser, and
	// nothing generator-internal appears at the call site.
	if !strings.Contains(out, "func findUser(ctx context.Context, db ") {
		t.Errorf("no execution API under the declared name:\n%s", out)
	}
	if !strings.Contains(out, "func buildFindUser(id int)") {
		t.Errorf("no builder wrapper:\n%s", out)
	}
	if !strings.Contains(out, "func _tinybindBuildfindUser(_b *") {
		t.Errorf("fragment builder missing:\n%s", out)
	}
	if strings.Contains(out, "func FindUser(") || strings.Contains(out, "func BuildFindUser(") {
		t.Errorf("a private statement leaked a public API:\n%s", out)
	}
}

func TestExportedStatementKeepsItsDeclaredName(t *testing.T) {
	source := "package store\ntype Row { id: int }\nexport statement FindUser(id: int): sql.one<Row> {SELECT id FROM users WHERE id = {id}}"
	out, err := generateSQL(t, source)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if !strings.Contains(out, "func FindUser(ctx context.Context, db ") {
		t.Errorf("exported statement is not callable under its declared name:\n%s", out)
	}
	if !strings.Contains(out, "func BuildFindUser(id int)") {
		t.Errorf("exported builder missing:\n%s", out)
	}
}

func TestFragmentStatementKeepsAnyNameCase(t *testing.T) {
	// A predicate is embedded into a caller's builder and never emitted under
	// its own name, so its case reaches no Go identifier and is not constrained.
	source := "package store\ntype Row { id: int }\nstatement Maybe(id: int): sql.predicate {id = {id}}\n" +
		"export statement FindUser(id: int): sql.one<Row> {SELECT id FROM users WHERE Maybe({id})}"
	if _, err := generateSQL(t, source); err != nil {
		t.Fatalf("generate: %v", err)
	}
}

func TestStatementNameCaseMustAgreeWithExport(t *testing.T) {
	for _, test := range []struct{ source, want string }{
		{
			"package store\ntype Row { id: int }\nexport statement findUser(id: int): sql.one<Row> {SELECT id FROM users WHERE id = {id}}",
			"declared export but its name is unexported",
		},
		{
			"package store\ntype Row { id: int }\nstatement FindUser(id: int): sql.one<Row> {SELECT id FROM users WHERE id = {id}}",
			"has an exported name",
		},
	} {
		_, err := generateSQL(t, test.source)
		if err == nil {
			t.Errorf("%s: generated without a diagnostic", test.want)
			continue
		}
		if !strings.Contains(err.Error(), test.want) {
			t.Errorf("error = %v, want %q", err, test.want)
		}
	}
}

// TestPrivateStatementCompilesAndRuns is the requirement in its original form:
// a handler in the same package calls the statement by the name it was declared
// under, and the generated code compiles with that call in it.
func TestPrivateStatementCompilesAndRuns(t *testing.T) {
	source := []byte(`package fixture
type User { id: int, name: string }
statement findUser(id: int): sql.one<User> {SELECT id, name FROM users WHERE id = {id}}
statement listUsers(): sql.many<User> {SELECT id, name FROM users}
statement deleteUser(id: int): sql.exec {DELETE FROM users WHERE id = {id}}
export statement CountUsers(): sql.one<User> {SELECT id, name FROM users LIMIT 1}`)
	generated, err := sqlbind.Generate("users.tb.sql", source, sqlbind.GenerateOptions{Dialect: sqlbind.DialectPostgreSQL})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	caller := []byte(`package fixture

import (
	"context"
	"database/sql"
	"testing"
)

// handler stands in for application code: it names the statements exactly as
// the template declared them.
func handler(ctx context.Context, db *sql.DB, id int) (User, error) {
	if _, err := deleteUser(ctx, db, id); err != nil {
		return User{}, err
	}
	for user := range listUsers(ctx, db) {
		_ = user
	}
	statement, err := buildFindUser(id)
	if err != nil {
		return User{}, err
	}
	_ = statement
	return findUser(ctx, db, id)
}

func TestCompiles(t *testing.T) { _ = handler }
`)
	runGenerated(t, generated, caller)
}
