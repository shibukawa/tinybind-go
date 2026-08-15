package sqlbind_test

import (
	"go/format"
	"strings"
	"testing"

	sqlbind "github.com/shibukawa/tinybind-go/templates/sqlbind"
)

const bindingHead = "package app\n\n" +
	"external Norm(s: string): string\n\n" +
	"type UserRow {\n  id: string\n  name: string\n}\n\n"

func bindingSource(body string) []byte {
	return []byte(bindingHead + "export statement FindUser(name: string, flag: bool): sql.many<UserRow> {\n" + body + "\n}\n")
}

func generateBinding(t *testing.T, body string) string {
	t.Helper()
	generated, err := sqlbind.Generate("users.tb.sql", bindingSource(body), sqlbind.GenerateOptions{Dialect: sqlbind.DialectPostgreSQL})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	// The binding emits a Go statement rather than an expression, so whether the
	// emitted locals are legal Go is decided by the file parsing at all.
	if _, err := format.Source(generated); err != nil {
		t.Fatalf("generated Go is not parseable: %v\n%s", err, generated)
	}
	return string(generated)
}

func bindingError(t *testing.T, body string) string {
	t.Helper()
	if _, err := sqlbind.Generate("users.tb.sql", bindingSource(body), sqlbind.GenerateOptions{Dialect: sqlbind.DialectPostgreSQL}); err != nil {
		return err.Error()
	}
	t.Fatal("want a generation error")
	return ""
}

// Normalize once in Go, use the result in several parameter positions. Without
// a binding each mention emits its own call, exactly as in markup.
func TestSQLValueBindingCallsItsExternalOnce(t *testing.T) {
	generated := generateBinding(t, "{val key = Norm(name)}\nSELECT id, name FROM users WHERE name = {key} OR alias = {key}")
	if calls := strings.Count(generated, "Norm("); calls != 1 {
		t.Fatalf("want one Norm call, got %d:\n%s", calls, generated)
	}
	if args := strings.Count(generated, "b.Arg(key)"); args != 2 {
		t.Fatalf("want the bound local used twice, got %d:\n%s", args, generated)
	}
}

// Nothing is rewritten to give the binding a subtree: a control body is already
// a Go block, so the local falls out of scope where the template says it does.
func TestSQLBindingUsesTheGeneratedBlockAsItsScope(t *testing.T) {
	generated := generateBinding(t, "SELECT id, name FROM users WHERE {if flag}{val key = Norm(name)}a = {key}{else}b = {name}{/if}")
	inner := strings.Index(generated, "key := Norm(name)")
	closing := strings.Index(generated, "} else {")
	if inner < 0 || closing < 0 || inner > closing {
		t.Fatalf("the binding was not emitted inside the branch that declares it:\n%s", generated)
	}
}

// A binding nothing reads still calls its external every time the statement is
// built, and the result goes nowhere. Go would report the unused local, but
// against a line of emitted code rather than the template line that caused it,
// so the language says it first.
func TestSQLUnreadBindingIsRefused(t *testing.T) {
	message := bindingError(t, "{val key = Norm(name)}\nSELECT id, name FROM users WHERE a = {name}")
	if !strings.Contains(message, "val binding key is never read") {
		t.Fatalf("want the unread diagnostic, got %q", message)
	}
	if !strings.Contains(message, "users.tb.sql:") {
		t.Fatalf("the diagnostic does not point at the template: %q", message)
	}
}

// Nothing silences an unread local any more, because none reaches emission.
func TestSQLEmitsNoBlankAssignment(t *testing.T) {
	generated := generateBinding(t, "{val key = Norm(name)}\nSELECT id, name FROM users WHERE a = {key}")
	if strings.Contains(generated, "_ = ") {
		t.Fatalf("a blank assignment survived:\n%s", generated)
	}
}

// A value binding may not take a name already visible, here as in markup: the
// rule is the language's rather than either lowering's.
func TestSQLBindingCannotReuseAVisibleName(t *testing.T) {
	for name, body := range map[string]string{
		"an earlier binding": "{val key = Norm(name)}{val key = Norm(name)}\nSELECT id, name FROM users WHERE a = {key}",
		"a sibling binding":  "{val key = Norm(name), key = Norm(name)}\nSELECT id, name FROM users WHERE a = {key}",
		"an enclosing block": "{val key = Norm(name)}\nSELECT id, name FROM users WHERE {if flag}{val key = Norm(name)}a = {key}{else}b = {key}{/if}",
		"a parameter":        "{val name = Norm(name)}\nSELECT id, name FROM users WHERE a = {name}",
	} {
		t.Run(name, func(t *testing.T) {
			message := bindingError(t, body)
			if !strings.Contains(message, "reuses a name that is already visible here") {
				t.Fatalf("want the shadowing diagnostic, got %q", message)
			}
		})
	}
}

// The bindings of one directive are independent here too, since the rule is the
// shared parser's rather than either lowering's. Written as two directives the
// dependency is ordinary, and the locals come out in order.
func TestSQLBindingsOfOneDirectiveCannotDependOnEachOther(t *testing.T) {
	message := bindingError(t, "{val raw = Norm(name), key = Norm(raw)}\nSELECT id, name FROM users WHERE a = {key}")
	if !strings.Contains(message, "the bindings of one directive are independent") {
		t.Fatalf("want the independence diagnostic, got %q", message)
	}
	generated := generateBinding(t, "{val raw = Norm(name)}{val key = Norm(raw)}\nSELECT id, name FROM users WHERE a = {key}")
	if !strings.Contains(generated, "key := Norm(raw)") {
		t.Fatalf("the second binding did not read the first:\n%s", generated)
	}
}

// An unread binding beside a read one is still unread, which a comma list makes
// easy to write by accident.
func TestSQLUnreadBindingBesideAReadOneIsRefused(t *testing.T) {
	message := bindingError(t, "{val raw = Norm(name), key = Norm(name)}\nSELECT id, name FROM users WHERE a = {key}")
	if !strings.Contains(message, "val binding raw is never read") {
		t.Fatalf("want the unread sibling reported, got %q", message)
	}
}

// A binding read on one branch alone is read, since a branch is where the
// statement varies rather than where the binding stops existing.
func TestSQLBindingReadOnOneBranchIsRead(t *testing.T) {
	generateBinding(t, "{val key = Norm(name)}\nSELECT id, name FROM users WHERE {if flag}a = {key}{else}b = {name}{/if}")
}

// A Go keyword is a legal binding name in the template and has to survive
// becoming a local, since a SQL binding reaches a Go identifier directly.
func TestSQLBindingNamedAfterAGoKeywordIsEscaped(t *testing.T) {
	generated := generateBinding(t, "{val type = Norm(name)}\nSELECT id, name FROM users WHERE a = {type}")
	if !strings.Contains(generated, "_type := Norm(name)") {
		t.Fatalf("the keyword-named binding was not escaped:\n%s", generated)
	}
}

// The builder already returns an error, so a failing external needs no new
// plumbing: the statement stops being built and the caller sees why.
func TestSQLFailingExternalIsCheckedAtTheBinding(t *testing.T) {
	generated, err := sqlbind.Generate("users.tb.sql",
		bindingSource("{val key = Norm(name)}\nSELECT id, name FROM users WHERE a = {key}"),
		sqlbind.GenerateOptions{Dialect: sqlbind.DialectPostgreSQL, ErrorExternals: map[string]bool{"Norm": true}})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	for _, want := range []string{"key, err := Norm(name)", "if err != nil"} {
		if !strings.Contains(string(generated), want) {
			t.Fatalf("generated code is missing %q:\n%s", want, generated)
		}
	}
}

// The placement rule is the same in both formats, since it is about where a
// failure has somewhere to go rather than about either lowering.
func TestSQLFailingExternalIsRefusedOutsideABinding(t *testing.T) {
	_, err := sqlbind.Generate("users.tb.sql",
		bindingSource("SELECT id, name FROM users WHERE a = {Norm(name)}"),
		sqlbind.GenerateOptions{Dialect: sqlbind.DialectPostgreSQL, ErrorExternals: map[string]bool{"Norm": true}})
	if err == nil || !strings.Contains(err.Error(), "returns an error, so it can only be the whole value of a val binding") {
		t.Fatalf("want the placement diagnostic, got %v", err)
	}
}

// A project whose externals declare no error generates exactly what it
// generated before this existed.
func TestSQLATotalExternalIsUnchanged(t *testing.T) {
	body := "{val key = Norm(name)}\nSELECT id, name FROM users WHERE a = {key}"
	with, err := sqlbind.Generate("users.tb.sql", bindingSource(body),
		sqlbind.GenerateOptions{Dialect: sqlbind.DialectPostgreSQL, ErrorExternals: map[string]bool{}})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if string(with) != generateBinding(t, body) {
		t.Fatal("an empty error set changed the output")
	}
}
