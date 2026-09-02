package generator_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// writeTempModule writes a go.mod that replace-points at this module root so
// packages.Load can type-check temp packages that import tinybind-go.
func writeTempModule(t *testing.T, dir string) {
	t.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	mod := "module tempmod\n\n" +
		"go 1.25\n\n" +
		"require github.com/shibukawa/tinybind-go v0.0.0\n\n" +
		"replace github.com/shibukawa/tinybind-go => " + filepath.ToSlash(root) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o644); err != nil {
		t.Fatal(err)
	}
}

// skipWithoutToolchain skips a test that shells out to the Go toolchain.
//
// Those tests tidy, build, or test a temp module that replace-points at this
// one, so each pays for resolving and compiling the whole module. They are what
// makes this the slowest package in the tree — long enough that on a cold build
// cache, competing for cores with every other package, it reaches the default
// ten-minute timeout and fails without a single test failing.
//
// Short mode is the fast loop, not a reduced suite: nothing here is skipped in
// a full run, and a change to generated code still has to survive compiling it.
func skipWithoutToolchain(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("short mode: this test runs the Go toolchain against a temp module")
	}
}

// tidyTempModule gives the temp module a tidied go.mod and go.sum after its
// sources are written. It is one go mod tidy per test binary rather than one
// per test, see tidiedModule.
func tidyTempModule(t *testing.T, dir string) {
	t.Helper()
	skipWithoutToolchain(t)
	installTidiedModule(t, "tempmod", dir)
}

// tidiedModule is the result of one go mod tidy over a module that imports
// every public package of this repository, so each test copies a go.mod and
// go.sum that already cover whatever its sources import instead of resolving
// the module graph again. Resolving it is what made these tests cost seconds
// apiece: the graph is the same every time, and only the sources differ.
var tidiedModule struct {
	once     sync.Once
	mod, sum []byte
	err      error
}

// tidySuperset runs go mod tidy once, in a scratch module named module that
// replace-points at this repository and imports all of its public packages.
func tidySuperset(module string) (mod, sum []byte, err error) {
	root, err := filepath.Abs("..")
	if err != nil {
		return nil, nil, err
	}
	list := exec.Command("go", "list", "-f", "{{.ImportPath}}", "./...")
	list.Dir = root
	out, err := list.Output()
	if err != nil {
		return nil, nil, fmt.Errorf("go list: %w", err)
	}
	var imports strings.Builder
	for _, path := range strings.Fields(string(out)) {
		if strings.Contains(path, "/internal/") || strings.Contains(path, "/testdata/") ||
			strings.Contains(path, "/examples/") || strings.Contains(path, "/cmd/") {
			continue
		}
		fmt.Fprintf(&imports, "\t_ %q\n", path)
	}
	dir, err := os.MkdirTemp("", "tidy-superset")
	if err != nil {
		return nil, nil, err
	}
	defer os.RemoveAll(dir)
	gomod := "module " + module + "\n\ngo 1.25\n\nrequire github.com/shibukawa/tinybind-go v0.0.0\n\nreplace github.com/shibukawa/tinybind-go => " + filepath.ToSlash(root) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644); err != nil {
		return nil, nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, "imports.go"), []byte("package "+module+"\n\nimport (\n"+imports.String()+")\n"), 0o644); err != nil {
		return nil, nil, err
	}
	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = dir
	if out, err := tidy.CombinedOutput(); err != nil {
		return nil, nil, fmt.Errorf("go mod tidy: %w\n%s", err, out)
	}
	if mod, err = os.ReadFile(filepath.Join(dir, "go.mod")); err != nil {
		return nil, nil, err
	}
	if sum, err = os.ReadFile(filepath.Join(dir, "go.sum")); err != nil {
		return nil, nil, err
	}
	return mod, sum, nil
}

// installTidiedModule writes the cached go.mod and go.sum into dir, keeping
// the module path the test already declared there, since its sources may
// import their own packages by it.
func installTidiedModule(t *testing.T, module, dir string) {
	t.Helper()
	tidiedModule.once.Do(func() {
		tidiedModule.mod, tidiedModule.sum, tidiedModule.err = tidySuperset(module)
	})
	if tidiedModule.err != nil {
		t.Fatalf("tidy the shared module: %v", tidiedModule.err)
	}
	mod := tidiedModule.mod
	if existing, err := os.ReadFile(filepath.Join(dir, "go.mod")); err == nil {
		if line, _, ok := strings.Cut(string(existing), "\n"); ok && strings.HasPrefix(line, "module ") {
			mod = append([]byte(line+"\n"), mod[len("module "+module+"\n"):]...)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), mod, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.sum"), tidiedModule.sum, 0o644); err != nil {
		t.Fatal(err)
	}
}
