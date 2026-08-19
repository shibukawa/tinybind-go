package sqlbind_test

import (
	"fmt"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/sqlbind"
)

// A template whose expression calls an external the fixture package never
// defines. The generated call is a compile error, which is what lets a test
// read back the position the compiler reports for a template expression.
//
// Norm is called on line 5 and only there.
const positionSource = `package queries
type User { id: int, name: string }
external Norm(s: string): string
export statement Find(name: string): sql.one<User> {
SELECT id, name FROM users WHERE name = {Norm(name)}
}`

func TestLineDirectivesReportTheTemplateLine(t *testing.T) {
	generated, err := sqlbind.Generate("users.tb.sql", []byte(positionSource), sqlbind.GenerateOptions{
		Dialect:        sqlbind.DialectPostgreSQL,
		LineDirectives: true,
		OutputName:     "generated.go",
	})
	if err != nil {
		t.Fatal(err)
	}
	output := buildFixture(t, generated)
	if !strings.Contains(output, "users.tb.sql:5:") {
		t.Fatalf("compiler did not report the template line:\n%s", output)
	}
	if strings.Contains(output, "generated.go:") {
		t.Fatalf("compiler named the generated file for a template expression:\n%s", output)
	}
}

// Without the option the same template reports against the generated file, so
// the mapping is the option's doing and not something the emitter always did.
func TestWithoutLineDirectivesTheGeneratedFileIsReported(t *testing.T) {
	generated, err := sqlbind.Generate("users.tb.sql", []byte(positionSource), sqlbind.GenerateOptions{
		Dialect: sqlbind.DialectPostgreSQL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(generated), "//line ") {
		t.Fatalf("directives emitted with the option off:\n%s", generated)
	}
	output := buildFixture(t, generated)
	if !strings.Contains(output, "generated.go:") {
		t.Fatalf("compiler did not report the generated file:\n%s", output)
	}
}

// A restore directive has to name the line the reader is actually on. It is
// written before formatting, so nothing but a check against the final bytes
// proves the number survived go/format.
func TestRestoreDirectivesNameTheirOwnLine(t *testing.T) {
	generated, err := sqlbind.Generate("users.tb.sql", []byte(positionSource), sqlbind.GenerateOptions{
		Dialect:        sqlbind.DialectPostgreSQL,
		LineDirectives: true,
		OutputName:     "generated.go",
	})
	if err != nil {
		t.Fatal(err)
	}
	restore := regexp.MustCompile(`^//line generated\.go:(\d+)$`)
	found := 0
	for index, line := range strings.Split(string(generated), "\n") {
		match := restore.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		found++
		want := index + 2 // the directive is on line index+1 and maps the next
		got, err := strconv.Atoi(match[1])
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("restore on line %d names line %d, want %d", index+1, got, want)
		}
	}
	if found == 0 {
		t.Fatalf("no restore directive emitted:\n%s", generated)
	}
	if strings.Contains(string(generated), "tinybind_restore.go") {
		t.Fatalf("a restore was left unresolved:\n%s", generated)
	}
}

// An indented //line is an ordinary comment the compiler ignores without
// saying so, so the left margin is a correctness property rather than a style
// one. Formatting has to leave it there.
func TestDirectivesSurviveFormattingAtTheLeftMargin(t *testing.T) {
	generated, err := sqlbind.Generate("users.tb.sql", []byte(positionSource), sqlbind.GenerateOptions{
		Dialect:        sqlbind.DialectPostgreSQL,
		LineDirectives: true,
		OutputName:     "generated.go",
	})
	if err != nil {
		t.Fatal(err)
	}
	for index, line := range strings.Split(string(generated), "\n") {
		if strings.Contains(line, "//line ") && !strings.HasPrefix(line, "//line ") {
			t.Fatalf("line %d holds an indented directive: %q", index+1, line)
		}
		// The path has to be absolute. A relative one is resolved a second time
		// by go vet, against the directory of the file holding the directive,
		// and reported doubled.
		if path, ok := strings.CutPrefix(line, "//line "); ok && !filepath.IsAbs(path) &&
			!strings.HasPrefix(path, "generated.go:") {
			t.Fatalf("line %d states a relative path: %q", index+1, line)
		}
	}
	formatted, err := format.Source(generated)
	if err != nil {
		t.Fatal(err)
	}
	if string(formatted) != string(generated) {
		t.Fatalf("generated source is not gofmt-clean:\n%s", formatted)
	}
}

// buildFixture compiles generated in a throwaway module and returns what the
// compiler said. The source is expected not to build; a build that succeeds is
// the test's own bug, because the position under test would never be printed.
func buildFixture(t *testing.T, generated []byte) string {
	t.Helper()
	skipWithoutToolchain(t)
	dir := t.TempDir()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	goMod := fmt.Sprintf("module fixture\n\ngo 1.26\n\nrequire github.com/shibukawa/tinybind-go v0.0.0\nreplace github.com/shibukawa/tinybind-go => %s\n", root)
	for name, content := range map[string][]byte{"go.mod": []byte(goMod), "generated.go": generated} {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	command := exec.Command("go", "build", "./...")
	command.Dir = dir
	command.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("fixture built, so no position was reported:\n%s", generated)
	}
	return string(output)
}
