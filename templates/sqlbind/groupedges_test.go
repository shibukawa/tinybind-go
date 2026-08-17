package sqlbind_test

import (
	"strings"
	"testing"
)

// TestDroppedLeadingOperatorLeavesNoDoubleSpace closes the whitespace gap: an
// operator leading a branch must carry away the space that separated it from its
// operand, so the branch where it vanishes has no double space.
func TestDroppedLeadingOperatorLeavesNoDoubleSpace(t *testing.T) {
	forms := map[string]string{
		"enclosing": `{if flagA}x = {p}{/if} AND {if flagB}y = {q}{/if}`,
		"leading":   `{if flagA}x = {p}{/if} {if flagB}AND y = {q}{/if}`,
		"trailing":  `{if flagA}x = {p} AND{/if} {if flagB}y = {q}{/if}`,
	}
	runtimeTest := `package queries
import ("strings"; "testing")
func TestSpacing(t *testing.T) {
	cases := []struct{ a, b bool; want string }{
		{true, true, "SELECT id FROM t WHERE x = $1 AND y = $2"},
		{true, false, "SELECT id FROM t WHERE x = $1"},
		{false, true, "SELECT id FROM t WHERE y = $1"},
		{false, false, "SELECT id FROM t"},
	}
	for _, c := range cases {
		s, err := BuildQ(1, 2, c.a, c.b)
		if err != nil { t.Fatal(err) }
		// Exact text, not collapsed: the point of this test is the whitespace.
		got := strings.TrimSpace(s.SQL)
		if got != c.want {
			t.Errorf("a=%v b=%v: SQL = %q, want %q", c.a, c.b, got, c.want)
		}
		if strings.Contains(got, "  ") {
			t.Errorf("a=%v b=%v: double space in %q", c.a, c.b, got)
		}
	}
}`
	for name, predicate := range forms {
		t.Run(name, func(t *testing.T) {
			source := "package queries\ntype R { id: int }\n" +
				"export statement Q(p: int, q: int, flagA: bool, flagB: bool): sql.many<R> {SELECT id FROM t WHERE " +
				predicate + "}"
			buildSQL(t, source, runtimeTest)
		})
	}
}

// TestDroppedOperatorKeepsAuthoredLayout checks the other half of the whitespace
// rule: an operator that survives keeps the newline and indent the author wrote, and
// one that vanishes takes that whole run away rather than leaving a blank line.
func TestDroppedOperatorKeepsAuthoredLayout(t *testing.T) {
	source := "package queries\ntype R { id: int }\n" +
		"export statement Q(p: int, q: int, flagA: bool, flagB: bool): sql.many<R> {\n" +
		"SELECT id FROM t\nWHERE\n  {if flagA}x = {p}{/if}\n  AND {if flagB}y = {q}{/if}\n}"
	buildSQL(t, source, `package queries
import ("strings"; "testing")
func TestLayout(t *testing.T) {
	cases := []struct{ a, b bool; want string }{
		{true, true, "SELECT id FROM t\nWHERE\n  x = $1\n  AND y = $2"},
		{true, false, "SELECT id FROM t\nWHERE\n  x = $1"},
		{false, true, "SELECT id FROM t\nWHERE\n  y = $1"},
		{false, false, "SELECT id FROM t"},
	}
	for _, c := range cases {
		s, err := BuildQ(1, 2, c.a, c.b)
		if err != nil { t.Fatal(err) }
		if got := strings.TrimSpace(s.SQL); got != c.want {
			t.Errorf("a=%v b=%v: SQL = %q, want %q", c.a, c.b, got, c.want)
		}
	}
}`)
}

// TestCaseRejectsAnElidableFragment closes the CASE gap. A CASE arm is neither a
// clause nor a comma list, so nothing can be withheld with an absent fragment and
// the template is reported instead of rendering CASE WHEN THEN.
func TestCaseRejectsAnElidableFragment(t *testing.T) {
	refused := map[string]string{
		"empty when condition": `SELECT CASE WHEN {if flagA}a{/if} THEN 1 ELSE 0 END AS c FROM t`,
		"empty then result":    `SELECT CASE WHEN a THEN {if flagA}1{/if} ELSE 0 END AS c FROM t`,
		"empty operand":        `SELECT CASE WHEN a AND {if flagA}b{/if} THEN 1 ELSE 0 END AS c FROM t`,
		"inside a predicate":   `SELECT id FROM t WHERE CASE WHEN {if flagA}a{/if} THEN 1 ELSE 0 END = 1`,
	}
	for name, body := range refused {
		t.Run("refused/"+name, func(t *testing.T) {
			source := "package queries\ntype R { c: int }\n" +
				"export statement Q(flagA: bool): sql.many<R> {" + body + "}"
			_, err := generateSQL(t, source)
			if err == nil {
				t.Fatalf("should have been reported: %s", body)
			}
			if !strings.Contains(err.Error(), "CASE") {
				t.Fatalf("diagnostic should name CASE: %v", err)
			}
		})
	}

	// A condition whose branches both emit is not elidable, so it stays legal.
	t.Run("both branches emit", func(t *testing.T) {
		source := `package queries
type R { c: int }
export statement Q(flagA: bool): sql.many<R> {
SELECT c FROM t WHERE CASE WHEN a THEN {if flagA}1{else}2{/if} ELSE 0 END = 1
}`
		if _, err := generateSQL(t, source); err != nil {
			t.Fatalf("should generate: %v", err)
		}
	})
}

// TestCommaGroupRemainingClauses closes the last comma clauses: FROM, WITH, WINDOW,
// USING, and PARTITION BY now manage their commas the way SET and ORDER BY do.
func TestCommaGroupRemainingClauses(t *testing.T) {
	t.Run("from", func(t *testing.T) {
		buildSQL(t, `package queries
type R { id: int }
export statement Q(flagA: bool): sql.many<R> {
SELECT a.id FROM a{if flagA}, b{/if} WHERE a.f
}`, `package queries
import ("strings"; "testing")
func TestFrom(t *testing.T) {
	on, _ := BuildQ(true)
	if got := strings.Join(strings.Fields(on.SQL), " "); got != "SELECT a.id FROM a, b WHERE a.f" { t.Fatalf("true: %q", got) }
	off, _ := BuildQ(false)
	if got := strings.Join(strings.Fields(off.SQL), " "); got != "SELECT a.id FROM a WHERE a.f" { t.Fatalf("false: %q", got) }
}`)
	})

	t.Run("with", func(t *testing.T) {
		buildSQL(t, `package queries
type R { id: int }
export statement Q(flagA: bool): sql.many<R> {
WITH a AS (SELECT id FROM x){if flagA}, b AS (SELECT id FROM y){/if}
SELECT id FROM a
}`, `package queries
import ("strings"; "testing")
func TestWith(t *testing.T) {
	on, _ := BuildQ(true)
	if got := strings.Join(strings.Fields(on.SQL), " "); got != "WITH a AS (SELECT id FROM x), b AS (SELECT id FROM y) SELECT id FROM a" { t.Fatalf("true: %q", got) }
	off, _ := BuildQ(false)
	if got := strings.Join(strings.Fields(off.SQL), " "); got != "WITH a AS (SELECT id FROM x) SELECT id FROM a" { t.Fatalf("false: %q", got) }
}`)
	})

	// A PARTITION BY reachable by elision is one in a WINDOW clause. Inside an OVER
	// in the select list it sits in a result context, where a conditional item is
	// already refused by validateStaticResultShape before elision is consulted.
	t.Run("window and partition by", func(t *testing.T) {
		buildSQL(t, `package queries
type R { id: int, n: int }
export statement Q(flagA: bool): sql.many<R> {
SELECT id, row_number() OVER w AS n FROM t WINDOW w AS (PARTITION BY city{if flagA}, role{/if})
}`, `package queries
import ("strings"; "testing")
func TestPartition(t *testing.T) {
	on, _ := BuildQ(true)
	if got := strings.Join(strings.Fields(on.SQL), " "); got != "SELECT id, row_number() OVER w AS n FROM t WINDOW w AS (PARTITION BY city, role)" { t.Fatalf("true: %q", got) }
	off, _ := BuildQ(false)
	if got := strings.Join(strings.Fields(off.SQL), " "); got != "SELECT id, row_number() OVER w AS n FROM t WINDOW w AS (PARTITION BY city)" { t.Fatalf("false: %q", got) }
}`)
	})

	t.Run("using table list", func(t *testing.T) {
		buildSQL(t, `package queries
export statement D(id: int, flagA: bool): sql.exec {
DELETE FROM t USING a{if flagA}, b{/if} WHERE t.id = {id}
}`, `package queries
import ("strings"; "testing")
func TestUsing(t *testing.T) {
	on, _ := BuildD(1, true)
	if got := strings.Join(strings.Fields(on.SQL), " "); got != "DELETE FROM t USING a, b WHERE t.id = $1" { t.Fatalf("true: %q", got) }
	off, _ := BuildD(1, false)
	if got := strings.Join(strings.Fields(off.SQL), " "); got != "DELETE FROM t USING a WHERE t.id = $1" { t.Fatalf("false: %q", got) }
}`)
	})
}

// TestCommaGroupLeavesFunctionArgumentsAlone guards the reason commaParenPredecessors
// is a closed set: a function call sits at a comma group's own depth, and eliding one
// of its arguments would change the call's arity.
func TestCommaGroupLeavesFunctionArgumentsAlone(t *testing.T) {
	source := `package queries
export statement Up(id: int, n: string, flagA: bool): sql.exec {
UPDATE t SET n = coalesce({n}, 'x'), seen = now() WHERE id = {id} {if flagA}AND f{/if}
}`
	buildSQL(t, source, `package queries
import ("strings"; "testing")
func TestArgs(t *testing.T) {
	off, err := BuildUp(1, "v", false)
	if err != nil { t.Fatal(err) }
	if got := strings.Join(strings.Fields(off.SQL), " "); got != "UPDATE t SET n = coalesce($1, 'x'), seen = now() WHERE id = $2" {
		t.Fatalf("false: %q", got)
	}
}`)
}
