package sqlbind_test

import (
	"testing"

	"github.com/shibukawa/tinybind-go/templates/internal/syntax"
	"github.com/shibukawa/tinybind-go/templates/sqlbind"
)

func formatSourceWidth(t *testing.T, source string, width int) string {
	t.Helper()
	module, err := sqlbind.Parse("x.tb.sql", []byte(source))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, err := syntax.PrintModule(module, []syntax.RootPrinter{sqlbind.RootPrinter()}, syntax.PrintOptions{Width: width})
	if err != nil {
		t.Fatalf("print: %v", err)
	}
	return out
}

// rule:sql-template-layout control_flow.block: an if whose branches begin with
// clause keywords is a clause of its own. It used to be absorbed as an item of
// the clause before it and stayed inline whenever it fit the width.
func TestFormatClauseOpeningControlIsABlock(t *testing.T) {
	t.Parallel()
	source := "export statement Rows(flag: bool, ids: int[]): sql.many<Row> {\n" +
		"  SELECT id FROM rows {if flag} WHERE id IN ({for id in ids}{id},{/for}0) ORDER BY id {else} ORDER BY name {/if} LIMIT 10\n}\n"
	want := "export statement Rows(flag: bool, ids: int[]): sql.many<Row> {\n" +
		"  SELECT id\n" +
		"  FROM rows\n" +
		"  {if flag}\n" +
		"    WHERE id IN ({for id in ids}{id},{/for}0)\n" +
		"    ORDER BY id\n" +
		"  {else}\n" +
		"    ORDER BY name\n" +
		"  {/if}\n" +
		"  LIMIT 10\n" +
		"}\n"
	got := formatSource(t, source)
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if again := formatSource(t, got); again != got {
		t.Errorf("not idempotent:\n%s", again)
	}
	// A fragment branch is still a fragment: nothing about it opens a line.
	fragment := "export statement Rows(flag: bool): sql.many<Row> {\n  SELECT id\n  FROM rows\n  WHERE 1 = 1 {if flag} AND id > 0{/if}\n}\n"
	if got := formatSource(t, fragment); got != fragment {
		t.Errorf("fragment control moved:\n%s", got)
	}
}

// Whitespace before a closing marker stayed pending and was handed to the
// token after the marker, so {/for}0 became {/for} 0. When the printer itself
// had opened the line before the marker, the second pass differed from the
// first and the file never settled.
func TestFormatBranchEdgeWhitespaceStaysInside(t *testing.T) {
	t.Parallel()
	source := "export statement Rows(ids: int[]): sql.many<Row> {\n  SELECT id FROM rows WHERE id IN ({for id in ids}{id},{/for}0)\n}\n"
	for _, width := range []int{30, 100} {
		once := formatSourceWidth(t, source, width)
		twice := formatSourceWidth(t, once, width)
		if once != twice {
			t.Errorf("width %d not idempotent\nfirst:\n%s\nsecond:\n%s", width, once, twice)
		}
	}
	spaced := "export statement Rows(ids: int[]): sql.many<Row> {\n  SELECT id\n  FROM rows\n  WHERE\n    id IN (\n      {for id in ids}\n        {id},\n      {/for}0\n    )\n}\n"
	if got := formatSourceWidth(t, spaced, 30); got != spaced {
		t.Errorf("a space appeared after the closer:\n%s", got)
	}
}

// A boolean operator that opens a line wrote its own space and then the item's
// own leading space, so AND was followed by two.
func TestFormatBooleanOperatorHasOneSpace(t *testing.T) {
	t.Parallel()
	source := "export statement Rows(a: string, b: string): sql.many<Row> {\n  SELECT id FROM rows WHERE first_column_name = {a} AND second_column_name = {b} AND third_column_name = 'x' AND fourth_column_name IS NOT NULL\n}\n"
	want := "export statement Rows(a: string, b: string): sql.many<Row> {\n" +
		"  SELECT id\n" +
		"  FROM rows\n" +
		"  WHERE\n" +
		"    first_column_name = {a}\n" +
		"    AND second_column_name = {b}\n" +
		"    AND third_column_name = 'x'\n" +
		"    AND fourth_column_name IS NOT NULL\n" +
		"}\n"
	if got := formatSource(t, source); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
