package generator_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

func TestGenerateTemplatesDiscoversStandardExtensions(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"first.tb.html": `package fixture
export component Hello(name: string): html {<h1>Hello {name}</h1>}`,
		"second.tb.html": `package fixture
export component Bye(name: string): html {<p>Bye {name}</p>}`,
		"users.tb.sql": `package fixture
type User { id: int, name: string }
export statement FindUser(id: int): sql.optional<User> {SELECT id, name FROM users WHERE id = {id}}`,
		"ignored.html": `this is deliberately not a template`,
		"ignored.sql":  `this is deliberately not a template`,
	}
	for name, source := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	options := generator.DefaultOptions()
	options.SQLDialect = "postgresql"
	path, err := generator.New(options).GenerateTemplates(dir, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != generator.DefaultTemplatesName {
		t.Fatalf("path = %s", path)
	}
	generated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, symbol := range []string{"func Hello", "func Bye", "func BuildFindUser", "func FindUser"} {
		if !bytes.Contains(generated, []byte(symbol)) {
			t.Errorf("generated output lacks %q", symbol)
		}
	}
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	goMod := "module fixture\n\ngo 1.26\n\n" +
		"require github.com/shibukawa/tinybind-go v0.0.0\n\n" +
		"replace github.com/shibukawa/tinybind-go => " + root + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "test", "-mod=mod", ".")
	command.Dir = dir
	command.Env = append(os.Environ(), "GOWORK=off")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("combined generated templates do not compile: %v\n%s\n%s", err, output, generated)
	}
}

func TestGenerateCommandGeneratesTemplatesWithoutBinderStructs(t *testing.T) {
	dir := t.TempDir()
	source := []byte(`package fixture
export statement Ping(): sql.exec {SELECT 1}`)
	if err := os.WriteFile(filepath.Join(dir, "ping.tb.sql"), source, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module fixture\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	set := generator.MustCommandSet(generator.GenerateCommand(generator.DefaultOptions()))
	exit := set.Run(context.Background(), []string{"generate", "-dir", dir, "-openapi=false", "-sql-context-api", "-sql-dialect=postgresql"}, generator.CommandIO{
		Stdout: &stdout, Stderr: &stderr,
	})
	if exit != 0 {
		t.Fatalf("exit=%d stderr=%s", exit, stderr.String())
	}
	if !strings.Contains(stdout.String(), generator.DefaultTemplatesName) {
		t.Fatalf("stdout=%q", stdout.String())
	}
	generated, err := os.ReadFile(filepath.Join(dir, generator.DefaultTemplatesName))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(generated, []byte("func PingContext(ctx context.Context")) {
		t.Fatalf("Context API was not generated:\n%s", generated)
	}
}

func TestTemplateFilesDoesNotDescendOrMatchOrdinaryFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "child"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.tb.html", "b.tb.sql", "c.html", "d.sql", "child/e.tb.sql"} {
		path := filepath.Join(dir, name)
		if strings.Contains(name, "/") {
			path = filepath.Join(dir, "child", "e.tb.sql")
		}
		if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	files, err := generator.TemplateFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || filepath.Base(files[0]) != "a.tb.html" || filepath.Base(files[1]) != "b.tb.sql" {
		t.Fatalf("files=%v", files)
	}
}

func TestGenerateTemplatesUsesCustomFilePatterns(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"page.tmpl": `package fixture
export component Page(): html {<h1>custom</h1>}`,
		"query.sqlt": `package fixture
export statement Ping(): sql.exec {SELECT 1}`,
		"ignored.tb.html": `not parsed`,
		"ignored.tb.sql":  `not parsed`,
	}
	for name, source := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	opts := generator.DefaultOptions()
	opts.HTMLTemplatePattern = "*.tmpl"
	opts.SQLTemplatePattern = "*.sqlt"
	opts.SQLDialect = "postgresql"
	path, err := generator.New(opts).GenerateTemplates(dir, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	generated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, symbol := range []string{"func Page", "func Ping"} {
		if !bytes.Contains(generated, []byte(symbol)) {
			t.Errorf("generated output lacks %q", symbol)
		}
	}
}

func TestTemplateFilesWithPatternsRejectsInvalidOrOverlappingPatterns(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "page.tmpl"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := generator.TemplateFilesWithPatterns(dir, "[", "*.sql"); err == nil || !strings.Contains(err.Error(), "invalid HTML template pattern") {
		t.Fatalf("invalid pattern error = %v", err)
	}
	if _, err := generator.TemplateFilesWithPatterns(dir, "*.tmpl", "page.*"); err == nil || !strings.Contains(err.Error(), "matches both") {
		t.Fatalf("overlap error = %v", err)
	}
}

func TestGenerateCommandUsesCustomTemplatePatterns(t *testing.T) {
	dir := t.TempDir()
	source := []byte(`package fixture
export statement Ping(): sql.exec {SELECT 1}`)
	if err := os.WriteFile(filepath.Join(dir, "ping.query"), source, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module fixture\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	set := generator.MustCommandSet(generator.GenerateCommand(generator.DefaultOptions()))
	exit := set.Run(context.Background(), []string{
		"generate", "-dir", dir, "-openapi=false",
		"-html-template-pattern=*.page", "-sql-template-pattern=*.query", "-sql-dialect=postgresql",
	}, generator.CommandIO{Stdout: &stdout, Stderr: &stderr})
	if exit != 0 {
		t.Fatalf("exit=%d stderr=%s", exit, stderr.String())
	}
	generated, err := os.ReadFile(filepath.Join(dir, generator.DefaultTemplatesName))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(generated, []byte("func Ping")) {
		t.Fatalf("custom-pattern SQL template was not generated:\n%s", generated)
	}
}

func TestGenerateTemplatesUsesCustomSQLExecutorResolver(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "dbctx"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Generated SQL always references the module runtime package.
	moduleRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod": "module fixture\n\ngo 1.26\n\nrequire github.com/shibukawa/tinybind-go v0.0.0\n\nreplace github.com/shibukawa/tinybind-go => " + filepath.ToSlash(moduleRoot) + "\n",
		"query.tb.sql": `package fixture
type User { id: int }
export statement GetUser(id: int): sql.one<User> {SELECT id FROM users WHERE id = {id}}`,
		"dbctx/dbctx.go": `package dbctx
import (
    "context"
    "database/sql"
)
type ExecutorInterface interface {
    ExecContext(context.Context, string, ...any) (sql.Result, error)
    QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}
func Executor(context.Context) (ExecutorInterface, error) { return nil, nil }`,
	}
	for name, source := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	opts := generator.DefaultOptions()
	opts.SQLExecutorResolver = &generator.SymbolPattern{PackagePath: "fixture/dbctx", Name: "Executor"}
	opts.SQLDialect = "postgresql"
	path, err := generator.New(opts).GenerateTemplates(dir, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	generated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(generated, []byte(`_tinybindresolver "fixture/dbctx"`)) || !bytes.Contains(generated, []byte("func GetUserContext")) {
		t.Fatalf("custom resolver wrapper missing:\n%s", generated)
	}
	command := exec.Command("go", "test", "./...")
	command.Dir = dir
	command.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("custom resolver output does not compile: %v\n%s\n%s", err, output, generated)
	}
}
