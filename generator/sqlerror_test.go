package generator_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

// The generator scans the package's Go sources and hands the result to both
// compilers. The HTML half of that path is exercised by the route fixtures; the
// SQL half had no external at all in any fixture, so the field was filled and
// never observed to arrive.
//
// This walks the whole chain for SQL: a Go loader declaring a trailing error, a
// template binding it, and the emitted builder checking it.
func TestGeneratedSQLChecksAScannedErrorExternal(t *testing.T) {
	skipWithoutToolchain(t)
	dir := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// The scan reads this, and nothing in the template says the function can
	// fail — which is the point: the declaration is the same either way.
	write("norm.go", "package fixture\n\nfunc Norm(s string) (string, error) { return s, nil }\n")
	write("users.tb.sql", `package fixture
external Norm(s: string): string
type User { id: int, name: string }
export statement FindUser(name: string): sql.optional<User> {
SELECT id, name FROM users WHERE name = {val key = Norm(name)}{key}
}`)

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
	for _, want := range []string{"key, _err := Norm(name)", "if _err != nil"} {
		if !bytes.Contains(generated, []byte(want)) {
			t.Fatalf("generated output lacks %q:\n%s", want, generated)
		}
	}
}
