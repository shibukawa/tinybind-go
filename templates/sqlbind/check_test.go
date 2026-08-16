package sqlbind_test

import (
	"strings"
	"testing"

	sqlbind "github.com/shibukawa/tinybind-go/templates/sqlbind"
)

const checkHead = "package app\n\n" +
	"external Norm(s: string): string\n" +
	"external Authorize(s: string)\n\n" +
	"type UserRow {\n  id: string\n  name: string\n}\n\n"

func checkSource(body string) []byte {
	return []byte(checkHead + "export statement FindUser(name: string, flag: bool): sql.many<UserRow> {\n" + body + "\n}\n")
}

func generateCheck(t *testing.T, body string, options sqlbind.GenerateOptions) string {
	t.Helper()
	options.Dialect = sqlbind.DialectPostgreSQL
	generated, err := sqlbind.Generate("users.tb.sql", checkSource(body), options)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	return string(generated)
}

func checkError(t *testing.T, body string, options sqlbind.GenerateOptions) string {
	t.Helper()
	options.Dialect = sqlbind.DialectPostgreSQL
	if _, err := sqlbind.Generate("users.tb.sql", checkSource(body), options); err != nil {
		return err.Error()
	}
	t.Fatal("want a generation error")
	return ""
}

// The builder already returns an error, so a check needs no plumbing of its
// own: the statement stops being built where the directive stands.
func TestSQLCheckEmitsAnErrorCheckedCall(t *testing.T) {
	generated := generateCheck(t, "{check Authorize(name)}\nSELECT id, name FROM users WHERE name = {name}", sqlbind.GenerateOptions{})
	want := "if err := Authorize(name); err != nil {"
	if !strings.Contains(generated, want) {
		t.Fatalf("generated code is missing %q:\n%s", want, generated)
	}
}

// A checked call that also returns a value is asked only whether it failed.
func TestSQLCheckDiscardsADeclaredResult(t *testing.T) {
	generated := generateCheck(t, "{check Norm(name)}\nSELECT id, name FROM users WHERE name = {name}",
		sqlbind.GenerateOptions{ErrorExternals: map[string]bool{"Norm": true}})
	want := "if _, err := Norm(name); err != nil {"
	if !strings.Contains(generated, want) {
		t.Fatalf("generated code is missing %q:\n%s", want, generated)
	}
}

// The directive contributes no bytes to the statement, so the text around it
// carries the spacing unchanged.
func TestSQLCheckEmitsNoSQL(t *testing.T) {
	with := generateCheck(t, "SELECT id, name FROM users WHERE name = {check Authorize(name)}{name}", sqlbind.GenerateOptions{})
	without := generateCheck(t, "SELECT id, name FROM users WHERE name = {name}", sqlbind.GenerateOptions{})
	// Everything the builder is told to write, in order. The check sits between
	// two of these lines and must contribute none of its own.
	statement := func(generated string) string {
		var written []string
		for _, line := range strings.Split(generated, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "b.WriteString(") || strings.HasPrefix(line, "b.Arg(") {
				written = append(written, line)
			}
		}
		if len(written) == 0 {
			t.Fatalf("no statement text in:\n%s", generated)
		}
		return strings.Join(written, "\n")
	}
	if statement(with) != statement(without) {
		t.Fatalf("the check changed the statement:\nwith:\n%s\nwithout:\n%s", statement(with), statement(without))
	}
}

// A declaration with no result type is not a value, and a check directive is
// the only position it has.
func TestSQLValueLessExternalIsRefusedInEveryOtherPosition(t *testing.T) {
	for _, body := range []string{
		"SELECT id FROM users WHERE name = {Authorize(name)}",
		"{val ok = Authorize(name)}\nSELECT id FROM users WHERE name = {ok}",
		"SELECT id FROM users WHERE name = {Norm(Authorize(name))}",
	} {
		message := checkError(t, body, sqlbind.GenerateOptions{})
		if !strings.Contains(message, "Authorize declares no result") {
			t.Fatalf("%s: want the no-result diagnostic, got %q", body, message)
		}
		if !strings.Contains(message, "{check Authorize(...)}") {
			t.Fatalf("%s: the diagnostic does not say what to write instead: %q", body, message)
		}
	}
}

// A binding read by nothing but a check is read, so the loader the check exists
// to inspect is not reported dead.
func TestSQLBindingReadOnlyByACheckIsRead(t *testing.T) {
	generateCheck(t, "{val key = Norm(name)}{check Authorize(key)}\nSELECT id, name FROM users WHERE flag = {flag}", sqlbind.GenerateOptions{})
}

// A call with no result and no error has no outcome anything can observe.
func TestSQLCheckRefusesACallThatCannotFail(t *testing.T) {
	message := checkError(t, "{check Authorize(name)}\nSELECT id FROM users WHERE name = {name}",
		sqlbind.GenerateOptions{ErrorExternals: map[string]bool{"Norm": true}})
	if !strings.Contains(message, "returns nothing at all") {
		t.Fatalf("want the no-outcome diagnostic, got %q", message)
	}
}

// The other way to arrive at a call that cannot fail: one that answers a value
// and only a value. That is a binding, and the diagnostic says so.
func TestSQLCheckRefusesAValueThatCannotFail(t *testing.T) {
	message := checkError(t, "{check Norm(name)}\nSELECT id, name FROM users WHERE name = {name}",
		sqlbind.GenerateOptions{ErrorExternals: map[string]bool{}})
	if !strings.Contains(message, "returns a value and no error") {
		t.Fatalf("want the total-call diagnostic, got %q", message)
	}
}

// A statement writing no check generates exactly what it generated before the
// directive existed.
func TestSQLUnusedIsFree(t *testing.T) {
	generated := generateCheck(t, "SELECT id, name FROM users WHERE name = {name}", sqlbind.GenerateOptions{})
	if strings.Contains(generated, "err != nil { return err }") {
		t.Fatalf("a statement with no check emitted an error check:\n%s", generated)
	}
}

// Whether the emitted check actually stops the build and hands the caller its
// own error is a runtime fact, so the generated package is compiled and run.
func TestSQLGenerateAndRunACheck(t *testing.T) {
	source := []byte(`package queries
external Authorize(s: string)
type User { id: int, name: string }
export statement Find(name: string): sql.one<User> {
{check Authorize(name)}SELECT id, name FROM users WHERE name = {name}
}`)
	generated, err := sqlbind.Generate("users.tb.sql", source, sqlbind.GenerateOptions{
		Dialect:        sqlbind.DialectPostgreSQL,
		ErrorExternals: map[string]bool{"Authorize": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	runtimeTest := []byte(`package queries
import (
	"errors"
	"testing"
)

var refused = errors.New("not allowed")

func Authorize(s string) error {
	if s == "bad" { return refused }
	return nil
}

func TestCheck(t *testing.T) {
	statement, err := BuildFind("ok")
	if err != nil { t.Fatalf("allowed input failed: %v", err) }
	if len(statement.Args) != 1 || statement.Args[0] != "ok" { t.Fatalf("Args = %#v", statement.Args) }
	if _, err := BuildFind("bad"); !errors.Is(err, refused) {
		t.Fatalf("err = %v, want the check's own error", err)
	}
}`)
	runGenerated(t, generated, runtimeTest)
}
