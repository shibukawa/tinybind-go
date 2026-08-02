package parser_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shibukawa/tinybind-go/parser"
)

// TestParsePackageSurvivesUnparsableSource pins that ordering the syntax files
// by name tolerates a file the parser could not read. Such a file still arrives
// as a syntax entry, but it reports token.NoPos and the FileSet holds no handle
// to name it, which used to take the calling process down. An editor that has
// created a file but not yet written into it leaves exactly that.
func TestParsePackageSurvivesUnparsableSource(t *testing.T) {
	dir := t.TempDir()
	mod := "module tempmod\n\ngo 1.25\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o644); err != nil {
		t.Fatal(err)
	}
	src := "package sample\n\nfunc Hello() string { return \"hello\" }\n"
	if err := os.WriteFile(filepath.Join(dir, "hello.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "empty.go"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := parser.ParsePackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Routes) != 0 {
		t.Fatalf("routes: %+v", res.Routes)
	}
}
