package sqlbind_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/sqlbind"
)

func TestGenerateAndRunBuilder(t *testing.T) {
	source := []byte(`package queries
type User { id: int, name: string }
export statement Find(id: int, names: string[], active: bool): sql.one<User> {
SELECT id, name FROM users WHERE id = {id} AND name IN ({names}) {if active}AND active = {true}{/if}
}`)
	generated, err := sqlbind.Generate("users.tb.sql", source, sqlbind.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	runtimeTest := []byte(`package queries
import "testing"
func TestBuilder(t *testing.T) {
	statement, err := BuildFind(7, []string{"a", "b"}, true)
	if err != nil { t.Fatal(err) }
	if statement.SQL != "\nSELECT id, name FROM users WHERE id = $1 AND name IN ($2, $3) AND active = $4\n" { t.Fatalf("SQL = %q", statement.SQL) }
	if len(statement.Args) != 4 || statement.Args[0] != 7 || statement.Args[3] != true { t.Fatalf("Args = %#v", statement.Args) }
}`)
	runGenerated(t, generated, runtimeTest)
}

func TestGenerateExecMutationAndQuestionPlaceholders(t *testing.T) {
	source := []byte(`package queries
export statement Rename(id: int, name: string, enabled: bool): sql.exec {
UPDATE users SET name = {name} {if enabled}WHERE id = {id}{/if}
}`)
	generated, err := sqlbind.Generate("users.tb.sql", source, sqlbind.GenerateOptions{PlaceholderStyle: "question"})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(generated, []byte("b.WriteByte('?')")) {
		t.Fatalf("question placeholder not generated:\n%s", generated)
	}
	runGenerated(t, generated, nil)
}

func TestPredicateAndRelationCompositionSharePlaceholderOrder(t *testing.T) {
	source := []byte(`package queries
type User { id: int, name: string }
statement MinimumID(id: int): sql.predicate {id >= {id}}
statement ActiveUsers(minimum: int): sql.relation<User> {SELECT id, name FROM users WHERE {MinimumID(minimum)}}
export statement Find(name: string, minimum: int): sql.many<User> {
SELECT active_users.id, active_users.name FROM subquery ActiveUsers(minimum) AS active_users WHERE active_users.name = {name}
}`)
	generated, err := sqlbind.Generate("relations.tb.sql", source, sqlbind.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	runtimeTest := []byte(`package queries
import "testing"
func TestRelations(t *testing.T) {
	statement, err := BuildFind("Ada", 10)
	if err != nil { t.Fatal(err) }
	if !strings.Contains(statement.SQL, "FROM (SELECT id, name FROM users WHERE id >= $1) AS active_users") { t.Fatalf("SQL = %q", statement.SQL) }
	if !strings.Contains(statement.SQL, "active_users.name = $2") { t.Fatalf("SQL = %q", statement.SQL) }
	if len(statement.Args) != 2 || statement.Args[0] != 10 || statement.Args[1] != "Ada" { t.Fatalf("Args = %#v", statement.Args) }
}`)
	// Add the only companion import without coupling generated code to it.
	runtimeTest = bytes.Replace(runtimeTest, []byte(`import "testing"`), []byte("import (\n\t\"strings\"\n\t\"testing\"\n)"), 1)
	runGenerated(t, generated, runtimeTest)
}

func TestGenerateDiagnostics(t *testing.T) {
	tests := []struct{ source, want string }{
		{`statement Bad(id: int): sql.exec { DELETE FROM users }`, "require a WHERE"},
		{`statement Bad(id: int): sql.exec { SELECT $1 }`, "manual SQL placeholders"},
		{`statement Bad(value: Missing): sql.exec { SELECT {value} }`, "unknown type Missing"},
		{`statement Bad(values: string[]): sql.exec { SELECT {for value in values}{value}{/for} }`, "general SQL loops"},
		{`type Row { id: int } statement Loop(): sql.relation<Row> {SELECT id FROM subquery Loop() AS loop_rows}`, "recursive SQL composition"},
		{`type Row { id: int } export statement Bad(ok: bool): sql.many<Row> {SELECT id {if ok}, 2{/if} FROM rows}`, "runtime-conditional select columns"},
		{`type Row { id: int, name: string } export statement Bad(): sql.many<Row> {SELECT id FROM rows}`, "has 1 columns"},
		{`type Row { id: int } export statement Bad(): sql.many<Row> {SELECT other FROM rows}`, `column "other" does not match field "id"`},
	}
	for _, test := range tests {
		_, err := sqlbind.Generate("bad.tb.sql", []byte(test.source), sqlbind.GenerateOptions{})
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("error = %v, want %q", err, test.want)
		}
	}
}

func TestSQLLiteralsAndCommentsAreLossless(t *testing.T) {
	source := []byte("statement Safe(): sql.exec { SELECT '{not_template}', $$ {also_not} $$/** {still_not} */ -- $1 {comment}\n }")
	if _, err := sqlbind.Generate("safe.tb.sql", source, sqlbind.GenerateOptions{}); err != nil {
		t.Fatal(err)
	}
}

func runGenerated(t *testing.T, generated, runtimeTest []byte) {
	t.Helper()
	dir := t.TempDir()
	files := map[string][]byte{"go.mod": []byte("module fixture\n\ngo 1.26\n"), "generated.go": generated}
	if runtimeTest != nil {
		files["generated_test.go"] = runtimeTest
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	command := exec.Command("go", "test", ".")
	command.Dir = dir
	command.Env = append(os.Environ(), "GOWORK=off")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generated code failed: %v\n%s\n%s", err, output, generated)
	}
}
