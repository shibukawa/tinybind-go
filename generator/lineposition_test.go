package generator_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

// The combined file is the shape a package actually gets: two templates and one
// output. It is also where the restore line numbers are decided, since neither
// template knows where the other one's declarations land.
func writeLinePositionTemplates(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	files := map[string]string{
		"page.tb.html": `package fixture
export component Page(name: string): html {<h1>{name}</h1>}`,
		"users.tb.sql": `package fixture
type User { id: int, name: string }
export statement FindUser(id: int): sql.optional<User> {SELECT id, name FROM users WHERE id = {id}}`,
		"doc.go": "package fixture\n\nimport _ \"github.com/shibukawa/tinybind-go/htmlbind\"\n",
		"go.mod": "module fixture\n\ngo 1.26\n\n" +
			"require github.com/shibukawa/tinybind-go v0.0.0\n\n" +
			"replace github.com/shibukawa/tinybind-go => " + filepath.ToSlash(root) + "\n",
	}
	for name, source := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tidyTempModule(t, dir)
	return dir
}

func TestTemplateLineDirectivesReachTheCombinedFile(t *testing.T) {
	skipWithoutToolchain(t)
	dir := writeLinePositionTemplates(t)
	options := generator.DefaultOptions()
	options.SQLDialect = "postgresql"
	options.TemplateLineDirectives = true
	path, err := generator.New(options).GenerateTemplates(dir, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	generated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Combining reparses and reprints, which is exactly the step that used to
	// drop every comment that is not a declaration doc.
	//
	// The path is absolute: it is the only form go build and go vet both print
	// correctly, and asserting it here is what keeps a relative one from
	// creeping back in unnoticed.
	for _, name := range []struct {
		file string
		line int
	}{{"page.tb.html", 2}, {"users.tb.sql", 3}} {
		want := fmt.Sprintf("//line %s:%d", filepath.Join(dir, name.file), name.line)
		if !strings.Contains(string(generated), want) {
			t.Fatalf("combined file lost %s:\n%s", want, generated)
		}
	}
	if strings.Contains(string(generated), "tinybind_restore.go") {
		t.Fatalf("a restore survived combining unresolved:\n%s", generated)
	}
	assertRestoresNameTheirOwnLine(t, generated, generator.DefaultTemplatesName)
}

// Off is the default, and it has to mean the bytes nobody asked to change stay
// the bytes they were.
func TestTemplateLineDirectivesAreOffByDefault(t *testing.T) {
	skipWithoutToolchain(t)
	dir := writeLinePositionTemplates(t)
	options := generator.DefaultOptions()
	options.SQLDialect = "postgresql"
	path, err := generator.New(options).GenerateTemplates(dir, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	generated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(generated), "//line ") {
		t.Fatalf("directives emitted with the option off:\n%s", generated)
	}
}

// An artifact caller chooses the name it writes under, so the artifact leaves
// the restore name unfilled and states the line. Filling the name in must not
// disturb the line.
func TestArtifactsResolveLinesAndLeaveTheNameToTheCaller(t *testing.T) {
	skipWithoutToolchain(t)
	dir := writeLinePositionTemplates(t)
	options := generator.DefaultOptions()
	options.SQLDialect = "postgresql"
	options.TemplateLineDirectives = true
	artifacts, err := generator.New(options).GenerateArtifacts(context.Background(), generator.GenerateRequest{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, artifact := range artifacts {
		if artifact.Kind != generator.ArtifactHTMLTemplate && artifact.Kind != generator.ArtifactSQLTemplate {
			continue
		}
		checked++
		if !strings.Contains(string(artifact.Content), "tinybind_restore.go:") {
			t.Fatalf("%s holds no restore for the caller to name:\n%s", artifact.OutputBase, artifact.Content)
		}
		named := generator.ResolveTemplatePositions(artifact.Content, artifact.OutputBase+"_pw_gen.go")
		if strings.Contains(string(named), "tinybind_restore.go") {
			t.Fatalf("%s kept the synthetic name after resolving:\n%s", artifact.OutputBase, named)
		}
		assertRestoresNameTheirOwnLine(t, named, artifact.OutputBase+"_pw_gen.go")
	}
	if checked != 2 {
		t.Fatalf("expected one artifact per template, checked %d", checked)
	}
}

// Options are hashed into the generation stamp, so flipping this one has to
// count as an input change; otherwise requirement:incremental-generation would
// skip the run that was asked for precisely because the shape changed.
func TestFlippingTheOptionInvalidatesTheStamp(t *testing.T) {
	skipWithoutToolchain(t)
	dir := writeLinePositionTemplates(t)
	options := generator.DefaultOptions()
	options.SQLDialect = "postgresql"

	off, err := generator.New(options).GenerateTemplates(dir, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(off)
	if err != nil {
		t.Fatal(err)
	}

	options.TemplateLineDirectives = true
	on, err := generator.New(options).GenerateTemplates(dir, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(on)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(after), "//line ") {
		t.Fatalf("the second run kept the first run's output:\n%s", after)
	}
	if string(before) == string(after) {
		t.Fatal("output did not change when the option was turned on")
	}
}

// The reason the path is absolute, checked against both readers from the
// module root, which is where a wrong path shows.
//
// go build prints the directive string verbatim, so a bare base name gives
// users.tb.sql with no directory: it does not open from the module root, and it
// is ambiguous the moment two packages hold a template of the same name. go vet
// resolves a relative path a second time, against the directory of the file
// holding the directive, so a module-root-relative path is reported doubled as
// store/store/users.tb.sql. An absolute path is shortened by the toolchain
// against the working directory instead, so both readers print one path, and it
// is one that opens.
func TestBothReadersPrintAPathThatResolves(t *testing.T) {
	skipWithoutToolchain(t)
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	// A nested package, because at the module root a directoryless path and a
	// correct one are the same string.
	pkg := filepath.Join(dir, "store")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod": "module fixture\n\ngo 1.26\n\n" +
			"require github.com/shibukawa/tinybind-go v0.0.0\n\n" +
			"replace github.com/shibukawa/tinybind-go => " + filepath.ToSlash(root) + "\n",
		// Norm is declared and never implemented, so the generated call is a
		// compile error at a known template line.
		"store/users.tb.sql": `package store
type User { id: int, name: string }
external Norm(s: string): string
export statement Find(name: string): sql.one<User> {
SELECT id, name FROM users WHERE name = {Norm(name)}
}`,
		"store/doc.go": "package store\n",
	}
	for name, source := range files {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tidyTempModule(t, dir)

	options := generator.DefaultOptions()
	options.SQLDialect = "postgresql"
	options.TemplateLineDirectives = true
	if _, err := generator.New(options).GenerateTemplates(pkg, pkg, ""); err != nil {
		t.Fatal(err)
	}

	template := filepath.Clean(filepath.Join(pkg, "users.tb.sql"))
	for _, reader := range []string{"build", "vet"} {
		t.Run(reader, func(t *testing.T) {
			command := exec.Command("go", reader, "./...")
			command.Dir = dir
			command.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
			out, err := command.CombinedOutput()
			if err == nil {
				t.Fatalf("fixture came back clean, so no position was reported:\n%s", out)
			}
			output := string(out)

			reported := regexp.MustCompile(`([^\s:]*users\.tb\.sql):(\d+)`).FindStringSubmatch(output)
			if reported == nil {
				t.Fatalf("go %s did not name the template:\n%s", reader, output)
			}
			// Taken as written from the directory the tool ran in, the path has
			// to be the template. That is the property: not that it looks right,
			// but that it opens.
			opened := reported[1]
			if !filepath.IsAbs(opened) {
				opened = filepath.Join(dir, opened)
			}
			if got := filepath.Clean(opened); got != template {
				t.Fatalf("go %s reported %q, which resolves to %q rather than the template\n%s",
					reader, reported[1], got, output)
			}
			if _, err := os.Stat(opened); err != nil {
				t.Fatalf("go %s reported %q, which does not open: %v\n%s", reader, reported[1], err, output)
			}
			if strings.Contains(output, filepath.FromSlash("store/store")) {
				t.Fatalf("go %s reported a doubled path:\n%s", reader, output)
			}
			if strings.Contains(output, generator.DefaultTemplatesName) {
				t.Fatalf("go %s named the generated file for a template expression:\n%s", reader, output)
			}
		})
	}
}

func assertRestoresNameTheirOwnLine(t *testing.T, source []byte, name string) {
	t.Helper()
	restore := regexp.MustCompile(`^//line ` + regexp.QuoteMeta(name) + `:(\d+)$`)
	found := 0
	for index, line := range strings.Split(string(source), "\n") {
		match := restore.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		found++
		got, err := strconv.Atoi(match[1])
		if err != nil {
			t.Fatal(err)
		}
		if want := index + 2; got != want {
			t.Fatalf("restore on line %d names line %d, want %d", index+1, got, want)
		}
	}
	if found == 0 {
		t.Fatalf("no restore directive naming %s:\n%s", name, source)
	}
}
