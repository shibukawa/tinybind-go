package sqlbind_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/sqlbind"
)

// A statement is emitted as a real Go function, so its frames are the
// template's own. This is the half htmlbind cannot have: rendering there walks
// an instruction list inside the shared coordinator, and no directive on
// generated code moves a frame that is not in it. See
// requirement:render-error-positions.
func TestPanicInAStatementNamesTheTemplate(t *testing.T) {
	skipWithoutToolchain(t)
	source := []byte(`package queries
type User { id: int, name: string }
external Norm(s: string): string
export statement Find(name: string): sql.one<User> {
SELECT id, name FROM users WHERE name = {Norm(name)}
}`)
	generated, err := sqlbind.Generate("users.tb.sql", source, sqlbind.GenerateOptions{
		Dialect:        sqlbind.DialectPostgreSQL,
		LineDirectives: true,
		OutputName:     "generated.go",
	})
	if err != nil {
		t.Fatal(err)
	}
	// Norm is the template's own external, so panicking inside it puts the
	// generated statement function on the stack below it.
	support := []byte(`package queries

func Norm(s string) string { panic("boom") }
`)
	main := []byte(`package queries

import (
	"fmt"
	"runtime/debug"
)

func Trace() (stack string) {
	defer func() {
		if recover() != nil {
			stack = string(debug.Stack())
		}
	}()
	_, _ = BuildFind("x")
	return ""
}

func Report() { fmt.Println(Trace()) }
`)
	runner := []byte(`package queries

import "testing"

func TestStack(t *testing.T) { Report() }
`)
	dir := t.TempDir()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	goMod := fmt.Sprintf("module fixture\n\ngo 1.26\n\nrequire github.com/shibukawa/tinybind-go v0.0.0\nreplace github.com/shibukawa/tinybind-go => %s\n", root)
	files := map[string][]byte{
		"go.mod":            []byte(goMod),
		"generated.go":      generated,
		"support.go":        support,
		"main.go":           main,
		"generated_test.go": runner,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	command := exec.Command("go", "test", "-v", "-run", "TestStack", ".")
	command.Dir = dir
	command.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("fixture failed: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "users.tb.sql:5") {
		t.Fatalf("stack does not name the template line:\n%s", output)
	}
}
