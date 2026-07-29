package sqlbind_test

import (
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/sqlbind"
)

// mutationSource wraps statement declarations in a module that already declares
// the record and helpers the cases below reuse.
func mutationSource(declarations string) []byte {
	return []byte("package queries\ntype User { id: int, name: string }\n" + declarations)
}

func generates(t *testing.T, declarations string) {
	t.Helper()
	if _, err := sqlbind.Generate("mutation.tb.sql", mutationSource(declarations), sqlbind.GenerateOptions{Dialect: sqlbind.DialectPostgreSQL}); err != nil {
		t.Errorf("Generate(%s) = %v, want success", declarations, err)
	}
}

func diagnoses(t *testing.T, declarations, want string) {
	t.Helper()
	_, err := sqlbind.Generate("mutation.tb.sql", mutationSource(declarations), sqlbind.GenerateOptions{Dialect: sqlbind.DialectPostgreSQL})
	if err == nil {
		t.Errorf("Generate(%s) succeeded, want a diagnostic", declarations)
		return
	}
	if !strings.Contains(err.Error(), want) {
		t.Errorf("Generate(%s) = %v, want it to mention %q", declarations, err, want)
	}
}

const whereRequired = "require a WHERE clause"

func TestMutationRequiresTopLevelWhere(t *testing.T) {
	generates(t, `export statement A(id: int): sql.exec {DELETE FROM users WHERE id = {id}}`)
	diagnoses(t, `export statement A(): sql.exec {DELETE FROM users}`, whereRequired)

	// A WHERE inside a subquery belongs to that subquery. The outer DELETE has
	// none, so it would remove every row.
	diagnoses(t, `export statement A(): sql.exec {DELETE FROM users USING (SELECT id FROM staged WHERE staged.flag) s}`, whereRequired)
	generates(t, `export statement A(id: int): sql.exec {DELETE FROM users USING (SELECT id FROM staged WHERE staged.flag) s WHERE users.id = {id}}`)

	// A WHERE spelled inside a literal or a comment is not a WHERE clause.
	diagnoses(t, `export statement A(name: string): sql.exec {DELETE FROM users -- where id = 1
}`, whereRequired)
	diagnoses(t, `export statement A(): sql.exec {DELETE FROM users /* WHERE id = 1 */}`, whereRequired)
	generates(t, `export statement A(name: string): sql.exec {DELETE FROM users WHERE name = {name} AND note <> 'where'}`)
}

func TestMutationWhereMustBeNonEmptyOnEveryBranch(t *testing.T) {
	// The predicate disappears when the flag is false, leaving a bare DELETE.
	diagnoses(t, `export statement A(id: int, flag: bool): sql.exec {DELETE FROM users {if flag}WHERE id = {id}{/if}}`, whereRequired)
	diagnoses(t, `export statement A(id: int, flag: bool): sql.exec {DELETE FROM users WHERE {if flag}id = {id}{/if}}`, whereRequired)

	// Both branches emit a predicate, so the clause is never empty.
	generates(t, `export statement A(id: int, name: string, flag: bool): sql.exec {DELETE FROM users WHERE {if flag}id = {id}{else}name = {name}{/if}}`)
	generates(t, `export statement A(id: int, flag: bool): sql.exec {DELETE FROM users WHERE id = {id} {if flag}AND flag{/if}}`)

	// A clause that ends before anything fills it stays unproven; the tokens of
	// the following clause must not count as the predicate.
	diagnoses(t, `export statement A(id: int, flag: bool): sql.exec {DELETE FROM users WHERE {if flag}id = {id}{/if} RETURNING id}`, whereRequired)
	diagnoses(t, `export statement A(flag: bool): sql.exec {DELETE FROM users WHERE {if flag}flag{/if} ORDER BY id LIMIT 1}`, whereRequired)
}

func TestMutationWhereThroughPredicate(t *testing.T) {
	// A sql.predicate that always emits proves the clause.
	generates(t, `statement ByID(id: int): sql.predicate {id = {id}}
export statement A(id: int): sql.exec {DELETE FROM users WHERE {ByID(id)}}`)

	// A predicate that can emit nothing does not.
	diagnoses(t, `statement Maybe(id: int, flag: bool): sql.predicate {{if flag}id = {id}{/if}}
export statement A(id: int, flag: bool): sql.exec {DELETE FROM users WHERE {Maybe(id, flag)}}`, whereRequired)

	// A predicate whose branches both emit does.
	generates(t, `statement Either(id: int, name: string, flag: bool): sql.predicate {{if flag}id = {id}{else}name = {name}{/if}}
export statement A(id: int, name: string, flag: bool): sql.exec {DELETE FROM users WHERE {Either(id, name, flag)}}`)
}

// The proof runs for every cardinality, not only sql.exec. A mutation with
// RETURNING was previously unguarded.
func TestMutationSafetyAppliesToEveryCardinality(t *testing.T) {
	diagnoses(t, `export statement A(): sql.one<User> {DELETE FROM users RETURNING id, name}`, whereRequired)
	diagnoses(t, `export statement A(): sql.optional<User> {DELETE FROM users RETURNING id, name}`, whereRequired)
	diagnoses(t, `export statement A(): sql.many<User> {DELETE FROM users RETURNING id, name}`, whereRequired)
	diagnoses(t, `export statement A(name: string): sql.one<User> {UPDATE users SET name = {name} RETURNING id, name}`, whereRequired)
	generates(t, `export statement A(id: int, name: string): sql.one<User> {UPDATE users SET name = {name} WHERE id = {id} RETURNING id, name}`)
}

func TestUpdateSetListMustBeNonEmptyOnEveryBranch(t *testing.T) {
	generates(t, `export statement A(id: int, name: string): sql.exec {UPDATE users SET name = {name} WHERE id = {id}}`)
	diagnoses(t, `export statement A(id: int, name: string, flag: bool): sql.exec {UPDATE users SET {if flag}name = {name}{/if} WHERE id = {id}}`, "require a SET list")
	generates(t, `export statement A(id: int, name: string, flag: bool): sql.exec {UPDATE users SET name = {name} {if flag}, id = {id}{/if} WHERE id = {id}}`)
}

// A CTE body is its own nesting level: the outer statement is the WITH tail.
func TestMutationSafetyThroughWith(t *testing.T) {
	diagnoses(t, `export statement A(): sql.exec {WITH stale AS (SELECT id FROM users WHERE flag) DELETE FROM users}`, whereRequired)
	generates(t, `export statement A(): sql.exec {WITH stale AS (SELECT id FROM users WHERE flag) DELETE FROM users WHERE id IN (SELECT id FROM stale)}`)
	// The tail is a SELECT, so nothing to prove even though a CTE writes.
	generates(t, `export statement A(id: int): sql.many<User> {WITH gone AS (DELETE FROM users WHERE id = {id} RETURNING id, name) SELECT id, name FROM gone}`)
}

// Reads are untouched by the mutation proof.
func TestNonMutationsAreNotChecked(t *testing.T) {
	generates(t, `export statement A(): sql.many<User> {SELECT id, name FROM users}`)
	generates(t, `export statement A(name: string): sql.exec {INSERT INTO users (name) VALUES ({name})}`)
	generates(t, `export statement A(): sql.exec {TRUNCATE users}`)
}

// No mutation guard reaches generated code; the proof is a generation-time one.
func TestGeneratedCodeCarriesNoMutationGuard(t *testing.T) {
	generated, err := sqlbind.Generate("mutation.tb.sql", mutationSource(
		`export statement A(id: int, name: string): sql.exec {UPDATE users SET name = {name} WHERE id = {id}}`),
		sqlbind.GenerateOptions{Dialect: sqlbind.DialectPostgreSQL})
	if err != nil {
		t.Fatal(err)
	}
	for _, banned := range []string{"SafeMutation", "require a WHERE", "WHERE clause"} {
		if strings.Contains(string(generated), banned) {
			t.Errorf("generated code contains %q:\n%s", banned, generated)
		}
	}
}
