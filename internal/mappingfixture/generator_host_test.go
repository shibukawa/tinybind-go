//go:build !tinygo

// The generator tests live apart from the runtime ones because they reach the
// host Go toolchain: they shell out to `go mod tidy` and drive generator, which
// pulls go/types into whatever binary compiles this file. scripts/tinygo-check.sh
// selects the runtime tests by name, but -run filters at run time, not at compile
// time -- so without this tag TinyGo still has to compile go/types, and on Go 1.27
// that fails inside internal/runtime/maps against TinyGo's internal/abi. The tag
// keeps them host-only, which is what they always were.

package mappingfixture_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

// skipWithoutToolchain skips a test that shells out to the Go toolchain. Those
// tests tidy a temp module against this one, which costs seconds apiece. Short
// mode is the fast loop; a full run skips nothing.
func skipWithoutToolchain(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("short mode: this test runs the Go toolchain against a temp module")
	}
}

func writeTempModule(t *testing.T, dir string) {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	mod := "module tempmod\n\ngo 1.25\n\nrequire github.com/shibukawa/tinybind-go v0.0.0\n\nreplace github.com/shibukawa/tinybind-go => " + filepath.ToSlash(root) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGenerator_DiscoversDecodeEncode(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	src := `package sample

import "github.com/shibukawa/tinybind-go/jsonbind"

type Note struct {
	Text string ` + "`payload:\"text\"`" + `
}

func use() {
	_, _ = jsonbind.DecodeJSON[Note](nil)
	_ = jsonbind.EncodeJSON[Note](nil, Note{})
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	skipWithoutToolchain(t)
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy: %v\n%s", err, out)
	}
	plan, err := generator.AnalyzePackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, n := range plan.Discovered {
		if n == "Note" {
			found = true
		}
	}
	if !found {
		t.Fatalf("DecodeJSON/EncodeJSON discovery missing Note: %v", plan.Discovered)
	}
	code, err := generator.Emit(plan)
	if err != nil {
		t.Fatal(err)
	}
	s := string(code)
	for _, n := range []string{"RegisterDecode[Note]", "RegisterEncode[Note]", "decodeNoteJSON", "encodeNote"} {
		if !strings.Contains(s, n) {
			t.Fatalf("missing %q in generated code", n)
		}
	}
}

func TestGenerator_EmitsTypeSpecificNoReflect(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	// copy types into temp package
	src, err := os.ReadFile("types.go")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "types.go"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	skipWithoutToolchain(t)
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy: %v\n%s", err, out)
	}
	// package name in types.go is mappingfixture — keep it
	opts := generator.DefaultOptions()
	opts.GenerateAll = true
	out, err := generator.New(opts).Generate(dir, dir, "tinybind_gen.go")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	code := string(data)
	if !strings.Contains(code, "func bindCreateUserRequest") {
		t.Fatalf("missing bindCreateUserRequest in:\n%s", code)
	}
	if !strings.Contains(code, "func writeCreateUserResponse") {
		t.Fatalf("missing writeCreateUserResponse in:\n%s", code)
	}
	if !strings.Contains(code, "RegisterBind[CreateUserRequest]") {
		t.Fatalf("missing registration in:\n%s", code)
	}
	if !strings.Contains(code, "func bindUploadAvatarRequest") {
		t.Fatalf("missing bindUploadAvatarRequest in:\n%s", code)
	}
	if !strings.Contains(code, "httpbind.ReadJSONBody(r)") && !strings.Contains(code, "httpbind.ReadJSONBodyOwned(r)") {
		t.Fatalf("missing httpbind.ReadJSONBody in:\n%s", code)
	}
	if !strings.Contains(code, "httpbind.ReadFormBody(r,") {
		t.Fatalf("missing httpbind.ReadFormBody in:\n%s", code)
	}
	if strings.Contains(code, "\"reflect\"") || strings.Contains(code, "reflect.") {
		t.Fatalf("generated code must not use reflect:\n%s", code)
	}
	// field sources present as literals / calls
	for _, needle := range []string{
		`PathValue(r, "org_id")`,
		`HeaderValue(r, "Authorization")`,
		`QueryLookup(queryVals, "name")`,
		`QueryLookup(queryVals, "keyword")`,
		`fileBody["image"]`,
	} {
		if !strings.Contains(code, needle) {
			t.Fatalf("missing %s in generated code", needle)
		}
	}
}
