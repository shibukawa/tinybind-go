package contextscan

import (
	"os"
	"path/filepath"
	"testing"
)

// TestExternalsDetectsLeadingContext covers how an external opts into
// receiving the context: by declaring the parameter.
func TestExternalsDetectsLeadingContext(t *testing.T) {
	dir := t.TempDir()
	source := `package pages

import "context"

type loader struct{}

func LoadTags(ctx context.Context, id string) ([]string, error) { return nil, nil }

func LoadUser(id string) (string, error) { return "", nil }

func Trailing(id string, ctx context.Context) (string, error) { return "", nil }

func (l loader) Method(ctx context.Context) error { return nil }
`
	if err := os.WriteFile(filepath.Join(dir, "loaders.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	// A file that does not parse is skipped rather than failing generation,
	// because detection runs before the package compiles.
	if err := os.WriteFile(filepath.Join(dir, "broken.go"), []byte("package pages\nfunc ("), 0o644); err != nil {
		t.Fatal(err)
	}
	found, err := Externals(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !found["LoadTags"] {
		t.Error("a leading context.Context was not detected")
	}
	for _, name := range []string{"LoadUser", "Trailing", "Method"} {
		if found[name] {
			t.Errorf("%s was wrongly reported as taking a leading context", name)
		}
	}
}

// The import name decides, not the literal spelling, so an aliased import still
// opts in.
func TestExternalsHonorsAnAliasedContextImport(t *testing.T) {
	dir := t.TempDir()
	source := `package pages

import goctx "context"

func Aliased(ctx goctx.Context) string { return "" }
`
	if err := os.WriteFile(filepath.Join(dir, "aliased.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	found, err := Externals(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !found["Aliased"] {
		t.Error("an aliased context import was not detected")
	}
}

// And a file aliasing some other package to the name context does not opt in by
// accident, which matching the literal spelling could not tell apart.
func TestExternalsIgnoresAnUnrelatedPackageNamedContext(t *testing.T) {
	dir := t.TempDir()
	source := `package pages

import context "example.com/notcontext"

func Impostor(ctx context.Context) string { return "" }
`
	if err := os.WriteFile(filepath.Join(dir, "impostor.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	found, err := Externals(dir)
	if err != nil {
		t.Fatal(err)
	}
	if found["Impostor"] {
		t.Error("a package aliased to the name context was wrongly detected")
	}
}
