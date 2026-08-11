package generator_test

import (
	"os"
	"os/exec"
	"path/filepath"
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

// tidyTempModule runs go mod tidy after package sources are written.
func tidyTempModule(t *testing.T, dir string) {
	t.Helper()
	skipWithoutToolchain(t)
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy: %v\n%s", err, out)
	}
}
