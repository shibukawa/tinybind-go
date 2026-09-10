package htmlbind

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/internal/syntax"
)

func printWith(t *testing.T, source string, options syntax.PrintOptions) string {
	t.Helper()
	module, err := Parse("x.tb.html", []byte(source))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, err := syntax.PrintModule(module, []syntax.RootPrinter{RootPrinter()}, options)
	if err != nil {
		t.Fatalf("print: %v", err)
	}
	return out
}

var sourcePositionLine = regexp.MustCompile(`(?m)^.*\.tb\.html:\d+:\d+.*\n`)

// sameGeneration is rule:template-format-fidelity generation_equality, with
// the lines that name source positions removed, because moving things is what
// a formatter does.
func sameGeneration(t *testing.T, source, formatted string, preserve bool) {
	t.Helper()
	before, err := Generate("x.tb.html", []byte(source), GenerateOptions{PreserveWhitespace: preserve})
	if err != nil {
		t.Fatalf("generate source: %v", err)
	}
	after, err := Generate("x.tb.html", []byte(formatted), GenerateOptions{PreserveWhitespace: preserve})
	if err != nil {
		t.Fatalf("generate formatted: %v\n%s", err, formatted)
	}
	before, after = sourcePositionLine.ReplaceAll(before, nil), sourcePositionLine.ReplaceAll(after, nil)
	if !bytes.Equal(before, after) {
		t.Errorf("generated output changed\nformatted:\n%s", formatted)
	}
}

// A branch label opened a line even where the branch before it ended glued,
// and with collapse on that break renders as a space the source never had.
func TestFormatGluedLabelStaysGlued(t *testing.T) {
	t.Parallel()
	source := "export component P(x: bool): html {\n  <p>Some long leading text that pushes the line past the width limit<b>a</b>{if x}<i>bbbbbbbbbbbbbbbb</i>{else}<i>cccccccccccccccc</i>{/if}<b>d</b></p>\n}\n"
	got := printWith(t, source, syntax.PrintOptions{})
	if strings.Contains(got, "</i>\n") || !strings.Contains(got, "</i>{else}<i>") {
		t.Errorf("label was separated from its branch:\n%s", got)
	}
	sameGeneration(t, source, got, false)
	// Where the branch ends on a run, the label still takes its own line.
	spaced := "export component P(x: bool): html {\n  <div>\n    {if x}\n      <i>a long enough branch body that the block cannot stay flat</i>\n    {else}\n      <i>and another one on the other side of the label</i>\n    {/if}\n  </div>\n}\n"
	if got := printWith(t, spaced, syntax.PrintOptions{}); got != spaced {
		t.Errorf("spaced layout changed:\n%s", got)
	}
}

// In a free container the closer takes its own line like the children do.
func TestFormatFreeContainerCloserOpensLine(t *testing.T) {
	t.Parallel()
	source := "export component D(): html {\n  <!doctype html>\n  <html lang=\"en\"><head><meta charset=\"utf-8\"><title>t</title></head><body><p>hi</p></body></html>\n}\n"
	want := "export component D(): html {\n  <!doctype html>\n  <html lang=\"en\">\n    <head>\n      <meta charset=\"utf-8\">\n      <title>t</title>\n    </head>\n    <body><p>hi</p></body>\n  </html>\n}\n"
	got := printWith(t, source, syntax.PrintOptions{})
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	sameGeneration(t, source, got, false)
	glued := "export component D(): html {<!doctype html><html lang=\"en\"><head><title>t</title></head><body></body></html>}\n"
	got = printWith(t, glued, syntax.PrintOptions{})
	if !strings.HasSuffix(got, "  </html>\n}\n") {
		t.Errorf("closing brace rides the html closer:\n%s", got)
	}
	sameGeneration(t, glued, got, false)
}

// With collapse off the generator emits every run it is given, so the layout
// may not add one anywhere, the free positions included: a head written on one
// line stays on one line.
func TestFormatPreservedFreePositionsAreNotLaidOut(t *testing.T) {
	t.Parallel()
	for _, source := range []string{
		"export component D(): html {\n  <!doctype html>\n  <html lang=\"en\"><head><meta charset=\"utf-8\"><title>t</title></head><body><p>hi</p></body></html>\n}\n",
		"export component D(): html {\n  <!doctype html>\n  <html lang=\"en\">\n    <head>\n      <meta charset=\"utf-8\">\n      <title>t</title>\n    </head>\n    <body><p>hi</p></body>\n  </html>\n}\n",
		"export component H(): html {\n  <head><title>t</title><meta name=\"a\" content=\"b\"></head>\n  <p>hi</p>\n}\n",
		"export component T(): html {\n  <table><tr><td>a</td><td>b</td></tr></table>\n}\n",
	} {
		got := printWith(t, source, syntax.PrintOptions{PreserveWhitespace: true})
		if got != source {
			t.Errorf("preserved layout changed\n got: %q\nwant: %q", got, source)
		}
		sameGeneration(t, source, got, true)
	}
}
