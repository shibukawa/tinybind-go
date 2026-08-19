package htmlbind_test

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

	"github.com/shibukawa/tinybind-go/templates/htmlbind"
)

// A template calling an external the fixture package never defines. The
// generated call is a compile error, which is what lets a test read back the
// position the compiler reports for a template expression.
//
// Shout is called on line 10, inside a loop body, and only there. Inside,
// because a loop body is a nested instruction list with its own literal and its
// own restore, and that is the case a list parallel to the top-level Ops would
// have missed.
//
// The exact line matters: without per-line pinning the diagnostic lands on
// line 11, the offset of the failing instruction inside its span rather than
// the line the call is written on.
const positionSource = `package pages

type Row {
  name: string
}

external Shout(s: string): string

export component Page(rows: Row[]): html {
<ul>{for row in rows}<li>{Shout(row.name)}</li>{/for}</ul>
}`

const shoutLine = 10

func TestLineDirectivesReportTheTemplateLine(t *testing.T) {
	generated := generateWithPositions(t, true)
	output := buildFixture(t, generated)
	want := fmt.Sprintf("page.tb.html:%d:", shoutLine)
	if !strings.Contains(output, want) {
		t.Fatalf("compiler did not report %s:\n%s", want, output)
	}
	if strings.Contains(output, "generated.go:") {
		t.Fatalf("compiler named the generated file for a template expression:\n%s", output)
	}
}

// Without the option the same template reports against the generated file, so
// the mapping is the option's doing and not something the emitter always did.
func TestWithoutLineDirectivesTheGeneratedFileIsReported(t *testing.T) {
	generated := generateWithPositions(t, false)
	if strings.Contains(string(generated), "//line ") {
		t.Fatalf("directives emitted with the option off:\n%s", generated)
	}
	output := buildFixture(t, generated)
	if !strings.Contains(output, "generated.go:") {
		t.Fatalf("compiler did not report the generated file:\n%s", output)
	}
}

// The instruction after a nested list is scaffolding of the enclosing literal,
// and the closing brace of the plan is scaffolding too. Both are reported
// against the generated file, which only a restore per nested list achieves.
func TestRestoreDirectivesNameTheirOwnLine(t *testing.T) {
	generated := generateWithPositions(t, true)
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
	// One for the loop body and one for the plan's own list.
	if found < 2 {
		t.Fatalf("expected a restore per instruction list, got %d:\n%s", found, generated)
	}
	if strings.Contains(string(generated), "tinybind_restore.go") {
		t.Fatalf("a restore was left unresolved:\n%s", generated)
	}
}

// An indented //line is an ordinary comment the compiler ignores without
// saying so. An Ops element is indented, so the directive above it has to be
// pulled back to the margin and kept there by formatting.
func TestDirectivesSurviveFormattingAtTheLeftMargin(t *testing.T) {
	generated := generateWithPositions(t, true)
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

func generateWithPositions(t *testing.T, on bool) []byte {
	t.Helper()
	options := htmlbind.GenerateOptions{Package: "pages"}
	if on {
		options.LineDirectives = true
		options.OutputName = "generated.go"
	}
	result, err := htmlbind.GenerateModule("page.tb.html", []byte(positionSource), options)
	if err != nil {
		t.Fatal(err)
	}
	return result.GoSource
}

// buildFixture compiles generated in a throwaway module and returns what the
// compiler said. The source is expected not to build; a build that succeeds is
// the test's own bug, because the position under test would never be printed.
func buildFixture(t *testing.T, generated []byte) string {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain unavailable")
	}
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
