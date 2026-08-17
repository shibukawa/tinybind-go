package sqlbind_test

import (
	"testing"
)

// TestCommaGroupOrderBy checks that an ORDER BY manages its commas and drops its
// own keyword when every item is conditional.
func TestCommaGroupOrderBy(t *testing.T) {
	source := `package queries
type R { id: int }
export statement Q(flagA: bool, flagB: bool): sql.many<R> {
SELECT id FROM t ORDER BY {if flagA}name{/if}, {if flagB}city{/if}
}`
	runtimeTest := `package queries
import ("strings"; "testing")
func TestOrderBy(t *testing.T) {
	cases := []struct{ a, b bool; want string }{
		{true, true, "SELECT id FROM t ORDER BY name, city"},
		{true, false, "SELECT id FROM t ORDER BY name"},
		{false, true, "SELECT id FROM t ORDER BY city"},
		{false, false, "SELECT id FROM t"},
	}
	for _, c := range cases {
		s, err := BuildQ(c.a, c.b)
		if err != nil { t.Fatal(err) }
		if got := strings.Join(strings.Fields(s.SQL), " "); got != c.want {
			t.Errorf("a=%v b=%v: SQL = %q, want %q", c.a, c.b, got, c.want)
		}
	}
}`
	buildSQL(t, source, runtimeTest)
}

// TestCommaGroupGroupByAndTrailingComma covers a GROUP BY with a static leading
// item, so the joiner survives, and a trailing conditional that must not leave one.
func TestCommaGroupGroupByAndTrailingComma(t *testing.T) {
	source := `package queries
type R { id: int }
export statement Q(flagA: bool): sql.many<R> {
SELECT id FROM t GROUP BY id{if flagA}, city{/if} ORDER BY id
}`
	runtimeTest := `package queries
import ("strings"; "testing")
func TestGroupBy(t *testing.T) {
	on, _ := BuildQ(true)
	if got := strings.Join(strings.Fields(on.SQL), " "); got != "SELECT id FROM t GROUP BY id, city ORDER BY id" {
		t.Fatalf("true: %q", got)
	}
	off, _ := BuildQ(false)
	if got := strings.Join(strings.Fields(off.SQL), " "); got != "SELECT id FROM t GROUP BY id ORDER BY id" {
		t.Fatalf("false: %q", got)
	}
}`
	buildSQL(t, source, runtimeTest)
}

// TestCommaGroupSet checks that an UPDATE SET list manages its commas, and that
// rule:sql-static-mutation-safety still refuses one whose items are all conditional.
func TestCommaGroupSet(t *testing.T) {
	source := `package queries
export statement Up(id: int, n: string, c: string, flagA: bool, flagB: bool): sql.exec {
UPDATE users SET seen = now(){if flagA}, name = {n}{/if}{if flagB}, city = {c}{/if} WHERE id = {id}
}`
	runtimeTest := `package queries
import ("strings"; "testing")
func TestSet(t *testing.T) {
	cases := []struct{ a, b bool; want string }{
		{true, true, "UPDATE users SET seen = now(), name = $1, city = $2 WHERE id = $3"},
		{true, false, "UPDATE users SET seen = now(), name = $1 WHERE id = $2"},
		{false, true, "UPDATE users SET seen = now(), city = $1 WHERE id = $2"},
		{false, false, "UPDATE users SET seen = now() WHERE id = $1"},
	}
	for _, c := range cases {
		s, err := BuildUp(7, "n", "c", c.a, c.b)
		if err != nil { t.Fatal(err) }
		if got := strings.Join(strings.Fields(s.SQL), " "); got != c.want {
			t.Errorf("a=%v b=%v: SQL = %q, want %q", c.a, c.b, got, c.want)
		}
	}
}`
	buildSQL(t, source, runtimeTest)

	// An UPDATE whose SET items are all conditional stays a generation error: a
	// withheld comma fills nothing, so the clause is not provably non-empty.
	refuses(t, `package queries
export statement Up(id: int, n: string, flagA: bool): sql.exec {
UPDATE users SET {if flagA}name = {n}{/if} WHERE id = {id}
}`, "SET")
}

// TestCommaGroupInsert checks the VALUES tuple and the INSERT column list, which is
// the one comma group with no keyword of its own to open it.
func TestCommaGroupInsert(t *testing.T) {
	source := `package queries
export statement Add(id: int, n: string, c: string, withCity: bool): sql.exec {
INSERT INTO users (id, name{if withCity}, city{/if}) VALUES ({id}, {n}{if withCity}, {c}{/if})
}`
	runtimeTest := `package queries
import ("strings"; "testing")
func TestInsert(t *testing.T) {
	on, err := BuildAdd(1, "n", "c", true)
	if err != nil { t.Fatal(err) }
	if got := strings.Join(strings.Fields(on.SQL), " "); got != "INSERT INTO users (id, name, city) VALUES ($1, $2, $3)" {
		t.Fatalf("true: %q", got)
	}
	off, err := BuildAdd(1, "n", "c", false)
	if err != nil { t.Fatal(err) }
	if got := strings.Join(strings.Fields(off.SQL), " "); got != "INSERT INTO users (id, name) VALUES ($1, $2)" {
		t.Fatalf("false: %q", got)
	}
	if len(off.Args) != 2 { t.Fatalf("Args = %#v", off.Args) }
}`
	buildSQL(t, source, runtimeTest)
}

// TestCommaGroupLeavesOtherListsAlone checks that a comma in a clause left as text
// stays text, so a SELECT list and a function argument list are untouched.
func TestCommaGroupLeavesOtherListsAlone(t *testing.T) {
	source := `package queries
type R { id: int, n: string }
export statement Q(flagA: bool): sql.many<R> {
SELECT id, coalesce(n, 'x') AS n FROM a, b WHERE a.id = b.id {if flagA}AND a.f{/if}
}`
	runtimeTest := `package queries
import ("strings"; "testing")
func TestOther(t *testing.T) {
	off, err := BuildQ(false)
	if err != nil { t.Fatal(err) }
	if got := strings.Join(strings.Fields(off.SQL), " "); got != "SELECT id, coalesce(n, 'x') AS n FROM a, b WHERE a.id = b.id" {
		t.Fatalf("false: %q", got)
	}
}`
	buildSQL(t, source, runtimeTest)
}
