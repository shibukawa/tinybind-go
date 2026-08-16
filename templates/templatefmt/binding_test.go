package templatefmt_test

import (
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/templatefmt"
)

// The closerless spelling is the whole reason the construct is a statement, so
// the formatter has to keep it: printing a closer, or indenting what the binding
// scopes, would hand back a shape the author did not write and did not want.
//
// This works because the formatter reads the parser's output, which is flat. The
// HTML compiler folds the following siblings into the binding for its own
// lowering, and that rewritten tree never reaches here.
func TestFormatterKeepsAValueBindingFlat(t *testing.T) {
	sources := map[string]string{
		"card.tb.html": "package app\n\nexternal Norm(s: string): string\n\n" +
			"export component Card(id: string): html {\n" +
			"  {val a = Norm(id), b = Norm(id)}\n" +
			"  <h1>{a}</h1>\n" +
			"  <p>{b}</p>\n}\n",
		"users.tb.sql": "package app\n\nexternal Norm(s: string): string\n\n" +
			"type UserRow { id: string, name: string }\n\n" +
			"export statement FindUser(name: string): sql.many<UserRow> {\n" +
			"  {val key = Norm(name)} SELECT id, name FROM users WHERE name = {key}\n}\n",
	}
	for filename, source := range sources {
		t.Run(filename, func(t *testing.T) {
			formatted, err := templatefmt.Source(filename, []byte(source), templatefmt.Options{})
			if err != nil {
				t.Fatalf("format: %v", err)
			}
			out := string(formatted)
			if strings.Contains(out, "{/val}") {
				t.Fatalf("the formatter invented a closer:\n%s", out)
			}
			if !strings.Contains(out, "{val ") {
				t.Fatalf("the binding did not survive formatting:\n%s", out)
			}
			again, err := templatefmt.Source(filename, formatted, templatefmt.Options{})
			if err != nil {
				t.Fatalf("reformat: %v", err)
			}
			if string(again) != out {
				t.Fatalf("formatting a binding is not idempotent:\nfirst:\n%s\nsecond:\n%s", out, again)
			}
		})
	}
}

// A check is a binding minus the binding, so it prints the same way: a leaf,
// no closer, and no indentation for what follows it. Its declaration has to
// survive too — writing a result type back would name one the author did not.
func TestFormatterKeepsACheckFlat(t *testing.T) {
	sources := map[string]string{
		"card.tb.html": "package app\n\nexternal Authorize(id: string)\n\n" +
			"export component Card(id: string): html {\n" +
			"  {check Authorize(id)}\n" +
			"  <h1>ok</h1>\n}\n",
		"users.tb.sql": "package app\n\nexternal Authorize(s: string)\n\n" +
			"type UserRow { id: string, name: string }\n\n" +
			"export statement FindUser(name: string): sql.many<UserRow> {\n" +
			"  {check Authorize(name)} SELECT id, name FROM users WHERE name = {name}\n}\n",
	}
	for filename, source := range sources {
		t.Run(filename, func(t *testing.T) {
			formatted, err := templatefmt.Source(filename, []byte(source), templatefmt.Options{})
			if err != nil {
				t.Fatalf("format: %v", err)
			}
			out := string(formatted)
			if strings.Contains(out, "{/check}") {
				t.Fatalf("the formatter invented a closer:\n%s", out)
			}
			if !strings.Contains(out, "{check Authorize(") {
				t.Fatalf("the check did not survive formatting:\n%s", out)
			}
			if strings.Contains(out, "external Authorize(id: string):") || strings.Contains(out, "external Authorize(s: string):") {
				t.Fatalf("the formatter gave the declaration a result type:\n%s", out)
			}
			again, err := templatefmt.Source(filename, formatted, templatefmt.Options{})
			if err != nil {
				t.Fatalf("reformat: %v", err)
			}
			if string(again) != out {
				t.Fatalf("formatting a check is not idempotent:\nfirst:\n%s\nsecond:\n%s", out, again)
			}
		})
	}
}
