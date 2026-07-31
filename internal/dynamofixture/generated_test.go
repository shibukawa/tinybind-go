//go:build !tinygo

// Checks on the committed generated file. They read the source and re-run the
// generator, neither of which is a TinyGo target.

package dynamofixture_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

func TestGeneratedFileHasNoReflect(t *testing.T) {
	source, err := os.ReadFile("dynamobind_gen.go")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(source, []byte(`"reflect"`)) || bytes.Contains(source, []byte("reflect.")) {
		t.Fatal("the committed item codec reaches for reflect")
	}
}

// TestCommittedCodecIsCurrent fails when the checked-in file no longer matches
// what this generator emits, so a stale codec is a test failure rather than a
// wrong item.
func TestCommittedCodecIsCurrent(t *testing.T) {
	plan, err := generator.AnalyzeDynamoItemsWithOptions(".", generator.DefaultOptions())
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	want, err := generator.EmitDynamoItems(plan, true)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	got, err := os.ReadFile("dynamobind_gen.go")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.TrimSpace(got), bytes.TrimSpace(want)) {
		t.Fatalf("dynamobind_gen.go is out of date; regenerate it\n--- committed ---\n%s\n--- generated ---\n%s", got, want)
	}
}
