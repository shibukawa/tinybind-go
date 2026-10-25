package httpbind_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRuntimeMappingSources_NoFieldReflect walks shipped runtime mapping sources
// and ensures the reflect package is not imported anywhere; registry dispatch
// uses typeMarker keys instead of reflect.Type.
func TestRuntimeMappingSources_NoFieldReflect(t *testing.T) {
	root := "."
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(root, e.Name())
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, src, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range f.Imports {
			if imp.Path != nil && imp.Path.Value == `"reflect"` {
				t.Fatalf("%s imports reflect; runtime files must stay reflect-free", path)
			}
		}
	}
}
