package sqlbind

import "testing"

// parseBody returns the node tree of a single statement whose body is the given
// SQL, so access-mode tests run against the same trees the compiler sees.
func parseBody(t *testing.T, body string) []Node {
	t.Helper()
	source := []byte("package q\ntype R { id: int }\n" +
		"export statement S(id: int, flag: bool): sql.one<R> {\n" + body + "\n}")
	module, err := Parse("s.tb.sql", source)
	if err != nil {
		t.Fatalf("parse %q: %v", body, err)
	}
	for _, declaration := range module.Declarations {
		if d, ok := declaration.(*TemplateDecl); ok {
			nodes, ok := d.Body.([]Node)
			if !ok {
				t.Fatalf("statement body is %T", d.Body)
			}
			return nodes
		}
	}
	t.Fatalf("no statement declared in %q", body)
	return nil
}

func TestIsReadOnlyReadingStatements(t *testing.T) {
	for _, body := range []string{
		"SELECT id FROM users",
		"select id from users",
		"\n\t  SELECT id FROM users\n",
		"VALUES (1), (2)",
		"TABLE users",
		"SELECT id FROM users WHERE id = {id}",
		"SELECT id FROM users {if flag}AND flag{/if}",
		"SELECT id FROM users ORDER BY id LIMIT 1",
	} {
		if !isReadOnly(parseBody(t, body)) {
			t.Errorf("isReadOnly(%q) = false, want true", body)
		}
	}
}

func TestIsReadOnlyWritingStatements(t *testing.T) {
	for _, body := range []string{
		"INSERT INTO users (id) VALUES ({id}) RETURNING id",
		"UPDATE users SET flag = {flag} WHERE id = {id} RETURNING id",
		"DELETE FROM users WHERE id = {id} RETURNING id",
		"delete from users where id = {id} returning id",
		"MERGE INTO users USING staged ON users.id = staged.id",
		"TRUNCATE users",
		"CREATE TABLE users (id int)",
		"DROP TABLE users",
		"COPY users FROM STDIN",
		"CALL rebuild_users()",
		"GRANT SELECT ON users TO reader",
	} {
		if isReadOnly(parseBody(t, body)) {
			t.Errorf("isReadOnly(%q) = true, want false", body)
		}
	}
}

// A keyword inside a comment is not SQL syntax. These bodies all read.
func TestIsReadOnlyIgnoresComments(t *testing.T) {
	for _, body := range []string{
		"-- update users set flag = true\nSELECT id FROM users",
		"-- delete from users\n-- insert into users\nSELECT id FROM users",
		"/* delete from users */ SELECT id FROM users",
		"/* update users\n   set flag = true */\nSELECT id FROM users",
		"/* outer /* delete from users */ still a comment */ SELECT id FROM users",
		"SELECT id FROM users -- for update",
		"SELECT id FROM users /* for update */",
	} {
		if !isReadOnly(parseBody(t, body)) {
			t.Errorf("isReadOnly(%q) = false, want true", body)
		}
	}
}

// A keyword inside a literal or a quoted identifier is not SQL syntax. Backtick
// and double-quote forms let a column be named after a statement verb.
func TestIsReadOnlyIgnoresLiteralsAndQuotedIdentifiers(t *testing.T) {
	for _, body := range []string{
		"SELECT 'update users set flag = true' AS note FROM users",
		"SELECT id FROM users WHERE note = 'delete from users'",
		"SELECT id FROM users WHERE note = 'for update'",
		"SELECT id FROM users WHERE note = 'it''s for update'",
		`SELECT "update", "delete" FROM users`,
		"SELECT `update`, `delete` FROM users",
		"SELECT `for update` FROM users",
		`SELECT id FROM "insert into users"`,
		"SELECT $tag$ delete from users $tag$ AS note FROM users",
		"SELECT $$ for update $$ AS note FROM users",
	} {
		if !isReadOnly(parseBody(t, body)) {
			t.Errorf("isReadOnly(%q) = false, want true", body)
		}
	}
}

func TestIsReadOnlyLockingClause(t *testing.T) {
	writes := []string{
		"SELECT id FROM users FOR UPDATE",
		"SELECT id FROM users FOR NO KEY UPDATE",
		"SELECT id FROM users FOR SHARE",
		"SELECT id FROM users FOR KEY SHARE",
		"SELECT id FROM users WHERE id = {id} FOR UPDATE SKIP LOCKED",
		"SELECT id FROM users {if flag}FOR UPDATE{/if}",
	}
	for _, body := range writes {
		if isReadOnly(parseBody(t, body)) {
			t.Errorf("isReadOnly(%q) = true, want false", body)
		}
	}
	// A lock inside a subquery belongs to that subquery, not to this statement.
	reads := []string{
		"SELECT id FROM users WHERE id IN (SELECT id FROM staged FOR UPDATE)",
		"SELECT (SELECT id FROM staged FOR SHARE) AS other FROM users",
	}
	for _, body := range reads {
		if !isReadOnly(parseBody(t, body)) {
			t.Errorf("isReadOnly(%q) = false, want true", body)
		}
	}
}

func TestIsReadOnlyWithStatements(t *testing.T) {
	reads := []string{
		"WITH recent AS (SELECT id FROM users) SELECT id FROM recent",
		"with recent as (select id from users) select id from recent",
		"WITH RECURSIVE tree AS (SELECT id FROM users) SELECT id FROM tree",
		"WITH recent (id) AS (SELECT id FROM users) SELECT id FROM recent",
		"WITH a AS (SELECT 1), b AS (SELECT 2) SELECT id FROM a",
		"WITH a AS MATERIALIZED (SELECT 1) SELECT id FROM a",
		"WITH a AS NOT MATERIALIZED (SELECT 1) SELECT id FROM a",
		"WITH a AS (SELECT id FROM users WHERE note = 'delete from users') SELECT id FROM a",
		"WITH a AS (SELECT id FROM (SELECT id FROM users) inner_q) SELECT id FROM a",
	}
	for _, body := range reads {
		if !isReadOnly(parseBody(t, body)) {
			t.Errorf("isReadOnly(%q) = false, want true", body)
		}
	}
	writes := []string{
		"WITH moved AS (DELETE FROM users WHERE id = {id} RETURNING id) SELECT id FROM moved",
		"WITH added AS (INSERT INTO users (id) VALUES ({id}) RETURNING id) SELECT id FROM added",
		"WITH touched AS (UPDATE users SET flag = {flag} RETURNING id) SELECT id FROM touched",
		"WITH a AS (SELECT 1), b AS (DELETE FROM users RETURNING id) SELECT id FROM a",
		"WITH a AS (SELECT 1) DELETE FROM users WHERE id IN (SELECT id FROM a) RETURNING id",
		"WITH a AS (SELECT 1) SELECT id FROM a FOR UPDATE",
	}
	for _, body := range writes {
		if isReadOnly(parseBody(t, body)) {
			t.Errorf("isReadOnly(%q) = true, want false", body)
		}
	}
}

// The leading verb must not depend on a runtime branch.
func TestIsReadOnlyConditionalLeadingVerb(t *testing.T) {
	for _, body := range []string{
		"{if flag}SELECT id FROM users{else}SELECT id FROM staged{/if}",
		"{if flag}SELECT id FROM users{else}DELETE FROM users RETURNING id{/if}",
	} {
		if isReadOnly(parseBody(t, body)) {
			t.Errorf("isReadOnly(%q) = true, want false", body)
		}
	}
}

// An unresolvable body is a write, so a scanner that loses its place cannot
// report read-only.
func TestScanSQLTokensUnterminated(t *testing.T) {
	for _, sql := range []string{
		"SELECT id FROM users WHERE note = 'unterminated",
		`SELECT "unterminated FROM users`,
		"SELECT `unterminated FROM users",
		"SELECT id FROM users /* unterminated",
		"SELECT $tag$ unterminated FROM users",
	} {
		if _, ok := scanSQLTokens(sql); ok {
			t.Errorf("scanSQLTokens(%q) ok = true, want false", sql)
		}
	}
}

func TestScanSQLTokensSkipsPlaceholders(t *testing.T) {
	tokens, ok := scanSQLTokens("SELECT id FROM users WHERE id = $1 AND flag = $2")
	if !ok {
		t.Fatal("scanSQLTokens ok = false")
	}
	for _, token := range tokens {
		if token.text == "$1" || token.text == "$2" {
			t.Errorf("placeholder %q became a token", token.text)
		}
	}
}
