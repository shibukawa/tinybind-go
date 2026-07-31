package sqlbind_test

import "testing"

// The result list is read at the statement's own nesting level, so a subquery
// in the select list neither ends the list nor contributes columns.
func TestResultShapeIgnoresSubqueries(t *testing.T) {
	generates(t, `export statement A(): sql.one<User> {SELECT id, (SELECT name FROM other WHERE other.id = users.id) AS name FROM users}`)
	generates(t, `export statement A(): sql.one<User> {SELECT (SELECT id FROM other LIMIT 1) AS id, name FROM users}`)
	diagnoses(t, `export statement A(): sql.one<User> {SELECT (SELECT id FROM other LIMIT 1) AS id FROM users}`, "has 1 columns")
}

func TestResultShapeCountAndOrder(t *testing.T) {
	generates(t, `export statement A(): sql.one<User> {SELECT id, name FROM users}`)
	generates(t, `export statement A(): sql.one<User> {SELECT * FROM users}`)
	generates(t, `export statement A(): sql.one<User> {SELECT users.* FROM users}`)
	diagnoses(t, `export statement A(): sql.one<User> {SELECT id FROM users}`, "has 1 columns")
	diagnoses(t, `export statement A(): sql.one<User> {SELECT id, name, flag FROM users}`, "has 3 columns")
	diagnoses(t, `export statement A(): sql.one<User> {SELECT name, id FROM users}`, "does not match field")
}

func TestResultShapeQualifiers(t *testing.T) {
	generates(t, `export statement A(): sql.one<User> {SELECT DISTINCT id, name FROM users}`)
	generates(t, `export statement A(): sql.one<User> {SELECT ALL id, name FROM users}`)
	generates(t, `export statement A(): sql.one<User> {SELECT DISTINCT ON (id) id, name FROM users}`)
	diagnoses(t, `export statement A(): sql.one<User> {SELECT DISTINCT ON (id) id FROM users}`, "has 1 columns")
}

// A comma inside a literal is not a column separator, and the item keeps its
// full text so the alias is still readable.
func TestResultShapeLiteralsAndComments(t *testing.T) {
	generates(t, `export statement A(): sql.one<User> {SELECT 'a, b' AS id, name FROM users}`)
	generates(t, `export statement A(): sql.one<User> {SELECT id, name FROM users -- , extra
}`)
	generates(t, `export statement A(): sql.one<User> {SELECT id /* , extra */, name FROM users}`)
	diagnoses(t, `export statement A(): sql.one<User> {SELECT 'a, b' AS id FROM users}`, "has 1 columns")
}

// A WITH statement is resolved to its tail instead of being skipped.
func TestResultShapeThroughWith(t *testing.T) {
	generates(t, `export statement A(): sql.many<User> {WITH recent AS (SELECT id, name FROM users) SELECT id, name FROM recent}`)
	diagnoses(t, `export statement A(): sql.many<User> {WITH recent AS (SELECT id, name FROM users) SELECT id FROM recent}`, "has 1 columns")
	// The CTE body has two columns but the tail has one; only the tail counts.
	diagnoses(t, `export statement A(): sql.many<User> {WITH recent AS (SELECT id, name FROM users) SELECT name FROM recent}`, "has 1 columns")
}

// A select list with no FROM clause ends at the next top-level clause.
func TestResultShapeWithoutFrom(t *testing.T) {
	generates(t, `export statement A(id: int, name: string): sql.one<User> {SELECT {id} AS id, {name} AS name}`)
	diagnoses(t, `export statement A(id: int): sql.one<User> {SELECT {id} AS id LIMIT 1}`, "has 1 columns")
}

func TestResultShapeReturningList(t *testing.T) {
	generates(t, `export statement A(id: int): sql.one<User> {DELETE FROM users WHERE id = {id} RETURNING id, name}`)
	diagnoses(t, `export statement A(id: int): sql.one<User> {DELETE FROM users WHERE id = {id} RETURNING id}`, "has 1 columns")
	// A RETURNING inside a CTE body belongs to that body, not to the tail.
	generates(t, `export statement A(id: int): sql.many<User> {WITH gone AS (DELETE FROM users WHERE id = {id} RETURNING id, name) SELECT id, name FROM gone}`)
}
