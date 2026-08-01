package sqlbind_test

import (
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/internal/syntax"
	"github.com/shibukawa/tinybind-go/templates/sqlbind"
)

func formatSource(t *testing.T, source string) string {
	t.Helper()
	module, err := sqlbind.Parse("x.tb.sql", []byte(source))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, err := syntax.PrintModule(module, []syntax.RootPrinter{sqlbind.RootPrinter()}, syntax.PrintOptions{})
	if err != nil {
		t.Fatalf("print: %v", err)
	}
	return out
}

func TestFormatShortStatementStaysOnOneLine(t *testing.T) {
	got := formatSource(t, "export statement FindUser(id: int): sql.one<UserRow> {SELECT id, name FROM users WHERE id = {id}}")
	want := "export statement FindUser(id: int): sql.one<UserRow> {\n" +
		"  SELECT id, name\n" +
		"  FROM users\n" +
		"  WHERE id = {id}\n" +
		"}\n"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatIsIdempotent(t *testing.T) {
	sources := []string{
		"package fixture\ntype UserRow { id: int, name: string }\nexport statement FindUser(id: int): sql.one<UserRow> {SELECT id, name FROM users WHERE id = {id}}",
		"statement A(id: int, flag: bool): sql.exec {DELETE FROM users WHERE {if flag}id = {id}{else}name = 'x'{/if}}",
		"export statement Report(low: int): sql.many<Row> {WITH recent AS (SELECT id, total FROM orders WHERE total > {low}) SELECT r.id, r.total, u.name FROM recent r JOIN users u ON u.id = r.id AND u.active WHERE r.total > 0 ORDER BY r.total DESC}",
	}
	for _, source := range sources {
		once := formatSource(t, source)
		twice := formatSource(t, once)
		if once != twice {
			t.Fatalf("not idempotent\nfirst:\n%s\nsecond:\n%s", once, twice)
		}
	}
}

func TestFormatShowsLayout(t *testing.T) {
	got := formatSource(t, "export statement Report(low: int): sql.many<Row> {WITH recent AS (SELECT id, total FROM orders WHERE total > {low}) SELECT r.id, r.total, u.name FROM recent r JOIN users u ON u.id = r.id AND u.active WHERE r.total > 0 ORDER BY r.total DESC}")
	t.Logf("\n%s", got)
	if !strings.Contains(got, "WITH recent AS (") {
		t.Errorf("CTE head not laid out:\n%s", got)
	}
}

func TestFormatKeepsComments(t *testing.T) {
	got := formatSource(t, "// what this file is for\npackage fixture\n\n// find one user\nexport statement FindUser(id: int): sql.one<UserRow> {SELECT id FROM users WHERE id = {id}}")
	if !strings.Contains(got, "// what this file is for") || !strings.Contains(got, "// find one user") {
		t.Fatalf("comments lost:\n%s", got)
	}
}
