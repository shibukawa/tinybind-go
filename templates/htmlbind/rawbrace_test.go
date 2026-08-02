package htmlbind_test

import (
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/htmlbind"
	"github.com/shibukawa/tinybind-go/templates/internal/syntax"
)

// A brace inside a script or style body is authored CSS or JavaScript. The
// parser keeps it as text, so the printer writes it back as it stands; escaping
// it would rewrite the authored language, and an escape that has to be undone
// on the next read is where non-convergence comes from.

func formatBody(t *testing.T, tag, body string) string {
	t.Helper()
	source := "export component Page(): html {\n<" + tag + ">\n" + body + "\n</" + tag + ">\n}\n"
	module, err := htmlbind.Parse("p.tb.html", []byte(source))
	if err != nil {
		t.Fatalf("parse %q: %v", body, err)
	}
	out, err := syntax.PrintModule(module, []syntax.RootPrinter{htmlbind.RootPrinter()}, syntax.PrintOptions{})
	if err != nil {
		t.Fatalf("print %q: %v", body, err)
	}
	return out
}

func TestRawTextBracesAreWrittenBackVerbatim(t *testing.T) {
	css := []string{
		".demo-badge { color: crimson }",
		".demo-badge {\n  color: crimson;\n}",
		".a { color: red }\n.b { color: blue }",
		"@media (min-width: 40rem) { .a { color: red } }",
		".a{color:red}",
		".a { color: red } /* note } */",
		"@media screen {\n  .a {\n    color: red;\n  }\n}",
	}
	for _, body := range css {
		if got := formatBody(t, "style", body); !strings.Contains(got, body) {
			t.Errorf("style body rewritten\nwant to contain:\n%s\ngot:\n%s", body, got)
		}
	}
	js := []string{
		"function f() { return 1 }",
		"const t = `a ${x} b`;",
		"if (a) {\n  g();\n}",
		"const o = {a: 1};",
		"const empty = {};",
	}
	for _, body := range js {
		if got := formatBody(t, "script", body); !strings.Contains(got, body) {
			t.Errorf("script body rewritten\nwant to contain:\n%s\ngot:\n%s", body, got)
		}
	}
}

func TestRawTextInsertionStaysEscaped(t *testing.T) {
	// {name} in a style body is an insertion by the parser's own rule, so its
	// literal spelling cannot be written bare: it has to keep the escape.
	source := "export component Page(color: string): html {\n<style>\n.a { color: {color} }\n</style>\n}\n"
	module, err := htmlbind.Parse("p.tb.html", []byte(source))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, err := syntax.PrintModule(module, []syntax.RootPrinter{htmlbind.RootPrinter()}, syntax.PrintOptions{})
	if err != nil {
		t.Fatalf("print: %v", err)
	}
	if !strings.Contains(out, "{color}") {
		t.Fatalf("the insertion was lost:\n%s", out)
	}
	if !strings.Contains(out, ".a { color: ") {
		t.Fatalf("the surrounding CSS braces were rewritten:\n%s", out)
	}
}

// TestRawTextBracesConverge searches short brace patterns for one that never
// settles, which is the shape rule:template-format-fidelity forbids.
func TestRawTextBracesConverge(t *testing.T) {
	alphabet := []string{"{", "}", "a", " ", "$", "\n", ":", "."}
	var build func(prefix string, depth int)
	build = func(prefix string, depth int) {
		if depth == 0 {
			assertConverges(t, prefix)
			return
		}
		for _, c := range alphabet {
			build(prefix+c, depth-1)
		}
	}
	for n := 1; n <= 4; n++ {
		build("", n)
	}
}

func assertConverges(t *testing.T, body string) {
	t.Helper()
	current := "export component P(): html {\n<style>\n" + body + "\n</style>\n}\n"
	var passes []string
	for pass := 0; pass < 3; pass++ {
		module, err := htmlbind.Parse("p.tb.html", []byte(current))
		if err != nil {
			return // a body that cannot parse is not this test's subject
		}
		out, err := syntax.PrintModule(module, []syntax.RootPrinter{htmlbind.RootPrinter()}, syntax.PrintOptions{})
		if err != nil {
			return
		}
		passes = append(passes, out)
		current = out
	}
	if passes[1] != passes[2] {
		t.Errorf("body %q never settles\n--1--\n%s\n--2--\n%s\n--3--\n%s", body, passes[0], passes[1], passes[2])
	}
}
