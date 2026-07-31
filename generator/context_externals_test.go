package generator

import (
	"os"
	"path/filepath"
	"testing"
)

// TestContextExternalsDetectsLeadingContext covers how an async external opts
// into receiving the boundary context: by declaring the parameter.
func TestContextExternalsDetectsLeadingContext(t *testing.T) {
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
	found, err := contextExternals(dir)
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
