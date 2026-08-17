package sqlbind_test

import (
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/sqlbind"
)

// buildSQL generates a package from source and runs a Go test inside it that
// asserts SQL text and Args together, per rule:sql-predicate-group-elision.
func buildSQL(t *testing.T, source, runtimeTest string) {
	t.Helper()
	generated, err := sqlbind.Generate("q.tb.sql", []byte(source), sqlbind.GenerateOptions{Dialect: sqlbind.DialectPostgreSQL})
	if err != nil {
		t.Fatalf("generate: %v\n%s", err, source)
	}
	runGenerated(t, generated, []byte(runtimeTest))
}

// refuses asserts that a source is a generation diagnostic naming want.
func refuses(t *testing.T, source, want string) {
	t.Helper()
	_, err := sqlbind.Generate("q.tb.sql", []byte(source), sqlbind.GenerateOptions{Dialect: sqlbind.DialectPostgreSQL})
	if err == nil {
		t.Fatalf("should have been reported: %s", source)
	}
	if want != "" && !strings.Contains(err.Error(), want) {
		t.Fatalf("diagnostic should name %q: %v", want, err)
	}
}

// TestGroupElisionBranchMatrix covers every combination of a two-condition and a
// three-condition predicate, which is the verification requirement:
// sql-conditional-predicate-composition asks for. The template is the one the
// change request opens with: read with every condition true and the if wrappers
// deleted, it is the statement it renders.
func TestGroupElisionBranchMatrix(t *testing.T) {
	source := `package queries
type User { id: int, name: string, city: string, age: int }
export statement SearchUsers(
  name: string, city: string, minAge: int,
  hasName: bool, hasCity: bool, hasAge: bool, staffOnly: bool
): sql.many<User> {
SELECT id, name, city, age
FROM users
WHERE
  {if hasName}name LIKE {name}{/if}
  AND {if hasCity}city = {city}{/if}
  AND ({if hasAge}age >= {minAge}{/if} OR {if staffOnly}role = 'staff'{/if})
ORDER BY id
}`
	runtimeTest := `package queries
import ("strings"; "testing")

// where isolates the WHERE clause, collapsing whitespace so the assertion is
// about the operators and parentheses rather than about layout.
func where(sql string) string {
	start := strings.Index(sql, "WHERE")
	if start < 0 { return "" }
	rest := sql[start:]
	if end := strings.Index(rest, "ORDER BY"); end >= 0 { rest = rest[:end] }
	return strings.Join(strings.Fields(rest), " ")
}

func TestMatrix(t *testing.T) {
	cases := []struct{
		hasName, hasCity, hasAge, staffOnly bool
		want string
		args int
	}{
		{true, true, true, true, "WHERE name LIKE $1 AND city = $2 AND (age >= $3 OR role = 'staff')", 3},
		{true, true, true, false, "WHERE name LIKE $1 AND city = $2 AND (age >= $3)", 3},
		{true, true, false, true, "WHERE name LIKE $1 AND city = $2 AND (role = 'staff')", 2},
		{true, true, false, false, "WHERE name LIKE $1 AND city = $2", 2},
		{true, false, true, true, "WHERE name LIKE $1 AND (age >= $2 OR role = 'staff')", 2},
		{true, false, true, false, "WHERE name LIKE $1 AND (age >= $2)", 2},
		{true, false, false, true, "WHERE name LIKE $1 AND (role = 'staff')", 1},
		{true, false, false, false, "WHERE name LIKE $1", 1},
		{false, true, true, true, "WHERE city = $1 AND (age >= $2 OR role = 'staff')", 2},
		{false, true, true, false, "WHERE city = $1 AND (age >= $2)", 2},
		{false, true, false, true, "WHERE city = $1 AND (role = 'staff')", 1},
		{false, true, false, false, "WHERE city = $1", 1},
		{false, false, true, true, "WHERE (age >= $1 OR role = 'staff')", 1},
		{false, false, true, false, "WHERE (age >= $1)", 1},
		{false, false, false, true, "WHERE (role = 'staff')", 0},
		{false, false, false, false, "", 0},
	}
	for _, c := range cases {
		statement, err := BuildSearchUsers("n", "c", 21, c.hasName, c.hasCity, c.hasAge, c.staffOnly)
		if err != nil { t.Fatal(err) }
		label := func() string { return strings.Join(strings.Fields(statement.SQL), " ") }
		if got := where(statement.SQL); got != c.want {
			t.Errorf("%v/%v/%v/%v: WHERE = %q, want %q", c.hasName, c.hasCity, c.hasAge, c.staffOnly, got, c.want)
		}
		if len(statement.Args) != c.args {
			t.Errorf("%v/%v/%v/%v: Args = %#v, want %d", c.hasName, c.hasCity, c.hasAge, c.staffOnly, statement.Args, c.args)
		}
		if c.want == "" && strings.Contains(statement.SQL, "WHERE") {
			t.Errorf("all conditions false still emitted WHERE: %q", label())
		}
		for _, dangling := range []string{"( OR )", "(OR", "WHERE AND", "AND AND", "AND ()", "()"} {
			if strings.Contains(label(), dangling) {
				t.Errorf("%v/%v/%v/%v: %q survived in %q", c.hasName, c.hasCity, c.hasAge, c.staffOnly, dangling, label())
			}
		}
	}
}`
	buildSQL(t, source, runtimeTest)
}

// TestGroupElisionAcceptedOperatorPositions covers the three positions
// decision:sql-boundary-joiner-inference accepts. The canonical form puts the
// operator in the enclosing text; the two in-branch forms are what the
// documentation and the existing tests contain, so they must keep working.
func TestGroupElisionAcceptedOperatorPositions(t *testing.T) {
	forms := map[string]string{
		"enclosing": `{if flagA}x = {p}{/if} AND {if flagB}y = {q}{/if}`,
		"leading":   `{if flagA}x = {p}{/if} {if flagB}AND y = {q}{/if}`,
		"trailing":  `{if flagA}x = {p} AND{/if} {if flagB}y = {q}{/if}`,
	}
	runtimeTest := `package queries
import ("strings"; "testing")
func TestForm(t *testing.T) {
	cases := []struct{ a, b bool; want string; args int }{
		{true, true, "SELECT id FROM t WHERE x = $1 AND y = $2", 2},
		{true, false, "SELECT id FROM t WHERE x = $1", 1},
		{false, true, "SELECT id FROM t WHERE y = $1", 1},
		{false, false, "SELECT id FROM t", 0},
	}
	for _, c := range cases {
		statement, err := BuildQ(1, 2, c.a, c.b)
		if err != nil { t.Fatal(err) }
		if got := strings.Join(strings.Fields(statement.SQL), " "); got != c.want {
			t.Errorf("a=%v b=%v: SQL = %q, want %q", c.a, c.b, got, c.want)
		}
		if len(statement.Args) != c.args {
			t.Errorf("a=%v b=%v: Args = %#v, want %d", c.a, c.b, statement.Args, c.args)
		}
	}
}`
	for name, predicate := range forms {
		t.Run(name, func(t *testing.T) {
			source := "package queries\n" +
				"type R { id: int }\n" +
				"export statement Q(p: int, q: int, flagA: bool, flagB: bool): sql.many<R> {\n" +
				"SELECT id FROM t WHERE " + predicate + "\n}"
			buildSQL(t, source, runtimeTest)
		})
	}
}

// TestGroupElisionLeavesDataParensAlone checks the exactness rule that a
// parenthesis preceded by a word is a call or list paren rather than a group, so
// an IN list keeps its parentheses in every branch.
func TestGroupElisionLeavesDataParensAlone(t *testing.T) {
	source := `package queries
type R { id: int }
export statement Q(names: string[], n: int, flagA: bool): sql.many<R> {
SELECT id FROM t WHERE name IN ({names}) {if flagA}AND n = {n}{/if}
}`
	runtimeTest := `package queries
import ("strings"; "testing")
func TestParens(t *testing.T) {
	on, err := BuildQ([]string{"a", "b"}, 5, true)
	if err != nil { t.Fatal(err) }
	if got := strings.Join(strings.Fields(on.SQL), " "); got != "SELECT id FROM t WHERE name IN ($1, $2) AND n = $3" {
		t.Fatalf("true: %q", got)
	}
	off, err := BuildQ([]string{"a", "b"}, 5, false)
	if err != nil { t.Fatal(err) }
	if got := strings.Join(strings.Fields(off.SQL), " "); got != "SELECT id FROM t WHERE name IN ($1, $2)" {
		t.Fatalf("false: %q", got)
	}
	if len(off.Args) != 2 { t.Fatalf("Args = %#v", off.Args) }
}`
	buildSQL(t, source, runtimeTest)
}

// TestGroupElisionBetweenIsNotAJoiner checks that the AND closing a BETWEEN is
// part of that two-operand form. A whole BETWEEN inside a condition renders
// unchanged, and a BETWEEN split across a branch boundary is reported.
func TestGroupElisionBetweenIsNotAJoiner(t *testing.T) {
	source := `package queries
type R { id: int }
export statement Q(lo: int, hi: int, flagA: bool, flagB: bool): sql.many<R> {
SELECT id FROM t WHERE {if flagA}n BETWEEN {lo} AND {hi}{/if} {if flagB}AND flag{/if}
}`
	runtimeTest := `package queries
import ("strings"; "testing")
func TestBetween(t *testing.T) {
	cases := []struct{ a, b bool; want string }{
		{true, true, "SELECT id FROM t WHERE n BETWEEN $1 AND $2 AND flag"},
		{true, false, "SELECT id FROM t WHERE n BETWEEN $1 AND $2"},
		{false, true, "SELECT id FROM t WHERE flag"},
		{false, false, "SELECT id FROM t"},
	}
	for _, c := range cases {
		statement, err := BuildQ(1, 9, c.a, c.b)
		if err != nil { t.Fatal(err) }
		if got := strings.Join(strings.Fields(statement.SQL), " "); got != c.want {
			t.Errorf("a=%v b=%v: SQL = %q, want %q", c.a, c.b, got, c.want)
		}
	}
}`
	buildSQL(t, source, runtimeTest)

	// A BETWEEN whose closing AND sits inside a condition is broken on the false
	// path, and is reported rather than carried further.
	refuses(t, `package queries
type R { id: int }
export statement Q(lo: int, hi: int, hasHi: bool): sql.many<R> {
SELECT id FROM t WHERE n BETWEEN {lo} {if hasHi}AND {hi}{/if}
}`, "BETWEEN")
}

// TestGroupElisionUnbalancedBranchParens checks that a parenthesis pair opening
// inside one branch and closing outside it stays unresolvable rather than guessed.
func TestGroupElisionUnbalancedBranchParens(t *testing.T) {
	refuses(t, `package queries
type R { id: int }
export statement Q(p: int, flagA: bool): sql.many<R> {
SELECT id FROM t WHERE {if flagA}({/if}n = {p}
}`, "parenthesis")
}

// TestGroupElisionThroughPredicate checks that a sql.predicate which may emit
// nothing fills the caller's group only when it emits, since the callee writes
// into the caller's own Builder.
func TestGroupElisionThroughPredicate(t *testing.T) {
	source := `package queries
type R { id: int }
statement Active(on: bool): sql.predicate {{if on}active{/if}}
export statement Q(p: int, flagA: bool, flagB: bool): sql.many<R> {
SELECT id FROM t WHERE {Active(flagA)} {if flagB}AND n = {p}{/if}
}`
	runtimeTest := `package queries
import ("strings"; "testing")
func TestPredicate(t *testing.T) {
	cases := []struct{ a, b bool; want string }{
		{true, true, "SELECT id FROM t WHERE active AND n = $1"},
		{true, false, "SELECT id FROM t WHERE active"},
		{false, true, "SELECT id FROM t WHERE n = $1"},
		{false, false, "SELECT id FROM t"},
	}
	for _, c := range cases {
		statement, err := BuildQ(7, c.a, c.b)
		if err != nil { t.Fatal(err) }
		if got := strings.Join(strings.Fields(statement.SQL), " "); got != c.want {
			t.Errorf("a=%v b=%v: SQL = %q, want %q", c.a, c.b, got, c.want)
		}
	}
}`
	buildSQL(t, source, runtimeTest)
}

// TestGroupElisionKeepsMutationProof checks that eliding an empty clause stays a
// SELECT affordance. rule:sql-static-mutation-safety must not weaken: a mutation
// predicate that can empty out is still a generation error, because dropping an
// empty WHERE there would turn one false branch into a full-table delete.
func TestGroupElisionKeepsMutationProof(t *testing.T) {
	refused := []string{
		`export statement A(id: int, flagA: bool): sql.exec {DELETE FROM users WHERE {if flagA}id = {id}{/if}}`,
		`export statement A(id: int, flagA: bool): sql.exec {DELETE FROM users {if flagA}WHERE id = {id}{/if}}`,
		`export statement A(id: int, flagA: bool): sql.exec {DELETE FROM users WHERE ({if flagA}id = {id}{/if})}`,
		`export statement A(id: int, flagA: bool, flagB: bool): sql.exec {DELETE FROM users WHERE {if flagA}id = {id}{/if} AND {if flagB}flag{/if}}`,
		`export statement A(id: int, flagA: bool): sql.exec {DELETE FROM users WHERE {if flagA}id = {id}{/if} RETURNING id}`,
	}
	for _, source := range refused {
		refuses(t, "package queries\n"+source, "")
	}
	accepted := []string{
		`export statement A(id: int, flagA: bool): sql.exec {DELETE FROM users WHERE id = {id} {if flagA}AND flag{/if}}`,
		`export statement A(id: int, name: string, flagA: bool): sql.exec {DELETE FROM users WHERE {if flagA}id = {id}{else}name = {name}{/if}}`,
		`export statement A(id: int, flagA: bool): sql.exec {DELETE FROM users WHERE (id = {id}) {if flagA}AND flag{/if}}`,
	}
	for _, source := range accepted {
		if _, err := sqlbind.Generate("q.tb.sql", []byte("package queries\n"+source), sqlbind.GenerateOptions{Dialect: sqlbind.DialectPostgreSQL}); err != nil {
			t.Errorf("mutation proof should still accept %s: %v", source, err)
		}
	}
}

// TestGroupElisionNestedGroupTakesItsJoiner checks that an empty nested group
// takes its parentheses and the joiner that attached it, which falls out of the
// frame protocol rather than needing a rule of its own.
func TestGroupElisionNestedGroupTakesItsJoiner(t *testing.T) {
	source := `package queries
type R { id: int }
export statement Q(p: int, q: int, flagA: bool, flagB: bool): sql.many<R> {
SELECT id FROM t WHERE n = {p} AND ({if flagA}x = {q}{/if} OR {if flagB}y{/if})
}`
	runtimeTest := `package queries
import ("strings"; "testing")
func TestNested(t *testing.T) {
	cases := []struct{ a, b bool; want string }{
		{true, true, "SELECT id FROM t WHERE n = $1 AND (x = $2 OR y)"},
		{true, false, "SELECT id FROM t WHERE n = $1 AND (x = $2)"},
		{false, true, "SELECT id FROM t WHERE n = $1 AND (y)"},
		{false, false, "SELECT id FROM t WHERE n = $1"},
	}
	for _, c := range cases {
		statement, err := BuildQ(1, 2, c.a, c.b)
		if err != nil { t.Fatal(err) }
		if got := strings.Join(strings.Fields(statement.SQL), " "); got != c.want {
			t.Errorf("a=%v b=%v: SQL = %q, want %q", c.a, c.b, got, c.want)
		}
	}
}`
	buildSQL(t, source, runtimeTest)
}

// TestGroupElisionHavingAndJoinOn checks the other two boolean clause openers. A
// bare ON is one of them: fmtclause.go already classifies it boolean, so the
// opener this needs was already computed.
func TestGroupElisionHavingAndJoinOn(t *testing.T) {
	source := `package queries
type R { id: int }
export statement Q(n: int, m: int, flagA: bool, flagB: bool): sql.many<R> {
SELECT t.id FROM t JOIN u ON t.u = u.id {if flagA}AND u.n = {n}{/if}
GROUP BY t.id HAVING {if flagB}count(*) > {m}{/if}
}`
	runtimeTest := `package queries
import ("strings"; "testing")
func TestOnHaving(t *testing.T) {
	cases := []struct{ a, b bool; want string }{
		{true, true, "SELECT t.id FROM t JOIN u ON t.u = u.id AND u.n = $1 GROUP BY t.id HAVING count(*) > $2"},
		{true, false, "SELECT t.id FROM t JOIN u ON t.u = u.id AND u.n = $1 GROUP BY t.id"},
		{false, true, "SELECT t.id FROM t JOIN u ON t.u = u.id GROUP BY t.id HAVING count(*) > $1"},
		{false, false, "SELECT t.id FROM t JOIN u ON t.u = u.id GROUP BY t.id"},
	}
	for _, c := range cases {
		statement, err := BuildQ(1, 2, c.a, c.b)
		if err != nil { t.Fatal(err) }
		if got := strings.Join(strings.Fields(statement.SQL), " "); got != c.want {
			t.Errorf("a=%v b=%v: SQL = %q, want %q", c.a, c.b, got, c.want)
		}
	}
}`
	buildSQL(t, source, runtimeTest)
}

// TestGroupElisionOnConflictKeepsItsOn checks that ON CONFLICT is a conflict
// action rather than a join predicate, so its ON opens no group. Treating it as one
// closes the group at CONFLICT with nothing having filled it, and the keyword
// vanishes with the group.
func TestGroupElisionOnConflictKeepsItsOn(t *testing.T) {
	source := `package queries
export statement Up(id: int, n: string, flagA: bool): sql.exec {
INSERT INTO t (id, n) VALUES ({id}, {n})
ON CONFLICT (id) DO UPDATE SET n = {n} WHERE t.id = {id} {if flagA}AND t.n <> {n}{/if}
}`
	runtimeTest := `package queries
import ("strings"; "testing")
func TestOnConflict(t *testing.T) {
	cases := []struct{ a bool; want string }{
		{true, "INSERT INTO t (id, n) VALUES ($1, $2) ON CONFLICT (id) DO UPDATE SET n = $3 WHERE t.id = $4 AND t.n <> $5"},
		{false, "INSERT INTO t (id, n) VALUES ($1, $2) ON CONFLICT (id) DO UPDATE SET n = $3 WHERE t.id = $4"},
	}
	for _, c := range cases {
		statement, err := BuildUp(1, "x", c.a)
		if err != nil { t.Fatal(err) }
		if got := strings.Join(strings.Fields(statement.SQL), " "); got != c.want {
			t.Errorf("a=%v: SQL = %q, want %q", c.a, got, c.want)
		}
	}
}`
	buildSQL(t, source, runtimeTest)
}

// TestGroupElisionDocumentedForm is the template docs/sqlbind.md teaches, which
// puts the operator inside the branch across several lines. It is the shape the
// existing documentation recommends, so it must keep rendering unchanged when the
// condition holds and lose the operator with the condition when it does not.
func TestGroupElisionDocumentedForm(t *testing.T) {
	source := `package queries
type User { id: int, name: string, active: bool }
export statement SearchUsers(name: string, activeOnly: bool): sql.many<User> {
SELECT id, name, active
FROM users
WHERE name = {name}
{if activeOnly}
  AND active = {true}
{/if}
ORDER BY id
}`
	runtimeTest := `package queries
import ("strings"; "testing")
func TestDocs(t *testing.T) {
	on, err := BuildSearchUsers("a", true)
	if err != nil { t.Fatal(err) }
	if got := strings.Join(strings.Fields(on.SQL), " "); got != "SELECT id, name, active FROM users WHERE name = $1 AND active = $2 ORDER BY id" {
		t.Fatalf("true: %q", got)
	}
	off, err := BuildSearchUsers("a", false)
	if err != nil { t.Fatal(err) }
	if got := strings.Join(strings.Fields(off.SQL), " "); got != "SELECT id, name, active FROM users WHERE name = $1 ORDER BY id" {
		t.Fatalf("false: %q", got)
	}
	if len(off.Args) != 1 { t.Fatalf("Args = %#v", off.Args) }
}`
	buildSQL(t, source, runtimeTest)
}

// TestGroupElisionByteIdentityWithoutConditions checks the compatibility claim
// that a body with nothing elidable in it is emitted exactly as before, so no
// group call reaches its generated code at all.
func TestGroupElisionByteIdentityWithoutConditions(t *testing.T) {
	source := `package queries
type R { id: int }
export statement Q(p: int): sql.many<R> {
SELECT id FROM t WHERE n = {p} AND (x OR y) ORDER BY id
}`
	generated, err := sqlbind.Generate("q.tb.sql", []byte(source), sqlbind.GenerateOptions{Dialect: sqlbind.DialectPostgreSQL})
	if err != nil {
		t.Fatal(err)
	}
	for _, call := range []string{"OpenGroup", "Joiner", "b.Item()", "CloseGroup"} {
		if strings.Contains(string(generated), call) {
			t.Errorf("a body with no condition should emit no %s call", call)
		}
	}
}
