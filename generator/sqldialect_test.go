package generator_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

func sqlTemplateDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	source := `package fixture
type User { id: int, name: string }
export statement FindUser(id: int): sql.optional<User> {SELECT id, name FROM users WHERE id = {id}}`
	if err := os.WriteFile(filepath.Join(dir, "users.tb.sql"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestSQLTemplatesRequireADialect covers the defect this option exists for: the
// placeholder style used to be unreachable from the generator, so every target
// silently received PostgreSQL placeholders.
func TestSQLTemplatesRequireADialect(t *testing.T) {
	for name, dialect := range map[string]string{"missing": "", "unknown": "oracle"} {
		t.Run(name, func(t *testing.T) {
			dir := sqlTemplateDir(t)
			options := generator.DefaultOptions()
			options.SQLDialect = dialect
			_, err := generator.New(options).GenerateTemplates(dir, dir, "")
			if err == nil {
				t.Fatal("generation succeeded without a usable dialect")
			}
			if !strings.Contains(err.Error(), "-sql-dialect") {
				t.Fatalf("error does not name the option: %v", err)
			}
			entries, readErr := os.ReadDir(dir)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if len(entries) != 1 {
				t.Fatalf("a rejected run wrote files: %v", entries)
			}
		})
	}
}

// TestHTMLOnlyPackageNeedsNoDialect keeps the requirement scoped: only a run
// that discovers a SQL template has to name a database.
func TestHTMLOnlyPackageNeedsNoDialect(t *testing.T) {
	dir := t.TempDir()
	source := `package fixture
export component Hello(name: string): html {<h1>Hello {name}</h1>}`
	if err := os.WriteFile(filepath.Join(dir, "hello.tb.html"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := generator.New(generator.DefaultOptions()).GenerateTemplates(dir, dir, ""); err != nil {
		t.Fatalf("HTML-only generation required a dialect: %v", err)
	}
}

func TestSQLDialectReachesGeneratedPlaceholders(t *testing.T) {
	for dialect, want := range map[string]string{
		"postgresql": "_tinybindsql.NewBuilder(_tinybindsql.Dollar)",
		"mysql":      "_tinybindsql.NewBuilder(_tinybindsql.Question)",
	} {
		t.Run(dialect, func(t *testing.T) {
			dir := sqlTemplateDir(t)
			options := generator.DefaultOptions()
			options.SQLDialect = dialect
			path, err := generator.New(options).GenerateTemplates(dir, dir, "")
			if err != nil {
				t.Fatal(err)
			}
			generated, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(generated, []byte(want)) {
				t.Fatalf("%s did not select %s:\n%s", dialect, want, generated)
			}
		})
	}
}

func TestGenerateCommandPassesTheSQLDialect(t *testing.T) {
	dir := sqlTemplateDir(t)
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module fixture\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	set := generator.MustCommandSet(generator.GenerateCommand(generator.DefaultOptions()))
	exit := set.Run(context.Background(), []string{
		"generate", "-dir", dir, "-openapi=false", "-sql-dialect=mysql",
	}, generator.CommandIO{Stdout: &stdout, Stderr: &stderr})
	if exit != 0 {
		t.Fatalf("exit=%d stderr=%s", exit, stderr.String())
	}
	generated, err := os.ReadFile(filepath.Join(dir, generator.DefaultTemplatesName))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(generated, []byte("_tinybindsql.NewBuilder(_tinybindsql.Question)")) {
		t.Fatalf("-sql-dialect=mysql did not reach the generated builder:\n%s", generated)
	}
}

// TestSQLDialectChangeInvalidatesTheStamp relies on the whole options value
// entering the generation fingerprint, so the dialect needs no hashed field of
// its own. A regression here would leave stale PostgreSQL output in place after
// a switch to MySQL.
func TestSQLDialectChangeInvalidatesTheStamp(t *testing.T) {
	_, fixture := copyCustomFrameworkFixture(t)
	request := stampRequest(fixture)

	options := customFrameworkOptions(t)
	first := generatePackage(t, generator.New(options), request)
	if first.Cached {
		t.Fatal("the first run reported a cache hit")
	}
	again := generatePackage(t, generator.New(options), request)
	if !again.Cached {
		t.Fatal("an unchanged rerun regenerated")
	}

	options.SQLDialect = "mysql"
	switched := generatePackage(t, generator.New(options), request)
	if switched.Cached {
		t.Fatal("a dialect change was treated as unchanged input")
	}
	generated, err := os.ReadFile(switched.TemplatesPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(generated, []byte("_tinybindsql.NewBuilder(_tinybindsql.Question)")) {
		t.Fatalf("regenerated output kept the previous dialect:\n%s", generated)
	}
}
