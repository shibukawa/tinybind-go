package externalscan

import (
	"os"
	"path/filepath"
	"testing"
)

// TestScanDetectsLeadingContext covers how an external opts into
// receiving the context: by declaring the parameter.
func TestScanDetectsLeadingContext(t *testing.T) {
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
	found, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !found.Context["LoadTags"] {
		t.Error("a leading context.Context was not detected")
	}
	for _, name := range []string{"LoadUser", "Trailing", "Method"} {
		if found.Context[name] {
			t.Errorf("%s was wrongly reported as taking a leading context", name)
		}
	}
}

// The import name decides, not the literal spelling, so an aliased import still
// opts in.
func TestScanHonorsAnAliasedContextImport(t *testing.T) {
	dir := t.TempDir()
	source := `package pages

import goctx "context"

func Aliased(ctx goctx.Context) string { return "" }
`
	if err := os.WriteFile(filepath.Join(dir, "aliased.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	found, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !found.Context["Aliased"] {
		t.Error("an aliased context import was not detected")
	}
}

// And a file aliasing some other package to the name context does not opt in by
// accident, which matching the literal spelling could not tell apart.
func TestScanIgnoresAnUnrelatedPackageNamedContext(t *testing.T) {
	dir := t.TempDir()
	source := `package pages

import context "example.com/notcontext"

func Impostor(ctx context.Context) string { return "" }
`
	if err := os.WriteFile(filepath.Join(dir, "impostor.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	found, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if found.Context["Impostor"] {
		t.Error("a package aliased to the name context was wrongly detected")
	}
}

// A trailing error is the other thing an implementation can declare that the
// template cannot. Unlike the context check it needs no import, so it is read
// from every file rather than only from those importing context.
func TestScanDetectsTrailingError(t *testing.T) {
	dir := t.TempDir()
	source := `package pages

func Load(id string) (string, error) { return "", nil }

func Fail(id string) error { return nil }

func Total(id string) string { return "" }

func Leading(id string) (error, string) { return nil, "" }

func (l loader) Method(id string) error { return nil }

type loader struct{}
`
	if err := os.WriteFile(filepath.Join(dir, "loaders.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	found, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Load", "Fail"} {
		if !found.Error[name] {
			t.Errorf("%s declares a trailing error and was not detected", name)
		}
	}
	for _, name := range []string{"Total", "Leading", "Method"} {
		if found.Error[name] {
			t.Errorf("%s was wrongly reported as returning an error", name)
		}
	}
	// The file imports no context, which used to end the scan for it.
	if len(found.Context) != 0 {
		t.Errorf("context map = %v, want empty", found.Context)
	}
}
