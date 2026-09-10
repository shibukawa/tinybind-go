package htmlbind_test

import (
	"testing"

	"github.com/shibukawa/tinybind-go/templates/htmlbind"
	"github.com/shibukawa/tinybind-go/templates/internal/syntax"
)

func formatPreserved(t *testing.T, source string) string {
	t.Helper()
	module, err := htmlbind.Parse("x.tb.html", []byte(source))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, err := syntax.PrintModule(module, []syntax.RootPrinter{htmlbind.RootPrinter()}, syntax.PrintOptions{PreserveWhitespace: true})
	if err != nil {
		t.Fatalf("print: %v", err)
	}
	return out
}

// With collapse off, every whitespace run the source has is output, so the
// layout copies runs rather than reshaping them. The run before an else was
// copied and then a line was opened on top of it, which put one blank line more
// before the label on every pass; the library's own idempotence guard turned
// that into a refusal to format the file at all.
func TestFormatPreservedBranchLabelsSettle(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		source string
	}{
		{
			"else after a spaced branch",
			"export component Page(ok: bool): html {\n  <div>\n    {if ok}\n      <a href=\"/x\">yes</a>\n    {else}\n      <b>no</b>\n    {/if}\n  </div>\n}\n",
		},
		{
			"else if chain",
			"export component Page(n: int): html {\n  <div>\n    {if n == 1}\n      <b>one</b>\n    {else if n == 2}\n      <b>two</b>\n    {else}\n      <b>many</b>\n    {/if}\n  </div>\n}\n",
		},
		{
			"fallback and recover",
			"external Load(id: int): Row\n\nexport component Page(id: int): html {\n  <div>\n    {await row = Load(id)}\n      <b>{row.title}</b>\n    {fallback}\n      <b>loading</b>\n    {recover err}\n      <b>{err.code}</b>\n    {/await}\n  </div>\n}\n",
		},
		{
			"else after a for",
			"export component Page(rows: int[]): html {\n  <div>\n    {if rows.length > 0}\n      {for row in rows}\n        <b>{row}</b>\n      {/for}\n    {else}\n      <b>none</b>\n    {/if}\n  </div>\n}\n",
		},
		{
			"a whitespace-only branch keeps its run",
			"export component Page(ok: bool): html {\n  <div>\n    {if ok}\n    {else}\n      <b>no</b>\n    {/if}\n  </div>\n}\n",
		},
		{
			"a glued label stays glued",
			"export component Page(ok: bool): html {\n  <div>\n    {if ok}<a href=\"/x\">yes, and this line is long enough that it cannot stay on one line with the rest</a>{else}<b>no</b>{/if}\n  </div>\n}\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := formatPreserved(t, tc.source)
			if got != tc.source {
				t.Errorf("format is not a fixed point\n got: %q\nwant: %q", got, tc.source)
			}
			if again := formatPreserved(t, got); again != got {
				t.Errorf("not idempotent\nfirst: %q\nsecond: %q", got, again)
			}
		})
	}
}
