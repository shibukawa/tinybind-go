package htmlbind_test

import (
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/htmlbind"
	"github.com/shibukawa/tinybind-go/templates/internal/syntax"
)

func formatSource(t *testing.T, source string) string {
	t.Helper()
	module, err := htmlbind.Parse("x.tb.html", []byte(source))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, err := syntax.PrintModule(module, []syntax.RootPrinter{htmlbind.RootPrinter()}, syntax.PrintOptions{})
	if err != nil {
		t.Fatalf("print: %v", err)
	}
	return out
}

func TestFormatHeadIsOneTagPerLine(t *testing.T) {
	got := formatSource(t, "export component Page(): html {<!DOCTYPE html><html lang=\"en\"><head><meta charset=\"utf-8\"/><title>t</title></head><body><p>hi</p></body></html>}")
	t.Logf("\n%s", got)
	for _, want := range []string{"<head>", "<meta charset=\"utf-8\"/>", "<title>t</title>", "</head>"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "<meta charset=\"utf-8\"/><title>") {
		t.Errorf("head children were not split:\n%s", got)
	}
}

func TestFormatKeepsGluedInlineRunOnOneLine(t *testing.T) {
	got := formatSource(t, "export component P(): html {<p><b>a</b><i>b</i></p>}")
	if !strings.Contains(got, "<p><b>a</b><i>b</i></p>") {
		t.Fatalf("glued run was split:\n%s", got)
	}
}

func TestFormatBreaksSpacedChildren(t *testing.T) {
	source := "export component P(): html {<div><section>first block of content here</section> <section>second block of content here</section> <section>third block of content goes here</section></div>}"
	got := formatSource(t, source)
	t.Logf("\n%s", got)
	if strings.Count(got, "<section>") != 3 {
		t.Fatalf("unexpected output:\n%s", got)
	}
	if !strings.Contains(got, "</section>\n") {
		t.Errorf("spaced children were not broken:\n%s", got)
	}
}

func TestFormatIsIdempotent(t *testing.T) {
	sources := []string{
		"export component Page(name: string): html {<h1>user {name}</h1><button server-action=\"Rename\" data-target=\"#name\">rename</button>}",
		"export component Page(): html {<!DOCTYPE html><html><head><title>x</title></head><body><ul><li><a href=\"/a\">a</a></li><li><a href=\"/b\">b</a></li></ul></body></html>}",
		"component Items(rows: Row[]): html {<ul>{for row in rows}<li>{row.name}</li>{/for}</ul>}",
		"component Flag(on: bool): html {<p>{if on}yes{else}no{/if}</p>}",
	}
	for _, source := range sources {
		once := formatSource(t, source)
		twice := formatSource(t, once)
		if once != twice {
			t.Fatalf("not idempotent\nfirst:\n%s\nsecond:\n%s", once, twice)
		}
	}
}

func TestFormatPreservesStyleBodyBytes(t *testing.T) {
	source := "export component Page(): html {<style>\n  body {{ margin: 0; }}\n</style>}"
	got := formatSource(t, source)
	if !strings.Contains(got, "body {{ margin: 0; }}") {
		t.Fatalf("style body was rewritten:\n%s", got)
	}
	if formatSource(t, got) != got {
		t.Fatalf("not idempotent:\n%s", got)
	}
}
