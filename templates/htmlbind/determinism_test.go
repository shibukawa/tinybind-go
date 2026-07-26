package htmlbind_test

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/htmlbind"
)

// TestGenerationIsDeterministic guards against emission that depends on map
// iteration order. A component call filling two slots used to number its fill
// plans from a map range, so the same template produced two different files at
// random and the golden comparison failed intermittently.
func TestGenerationIsDeterministic(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "templates", "htmlbind")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	var cases []string
	for _, entry := range entries {
		if entry.IsDir() {
			cases = append(cases, entry.Name())
		}
	}
	sort.Strings(cases)
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(root, name, "input.txt")
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			first, err := htmlbind.Generate(path, source, htmlbind.GenerateOptions{})
			if err != nil {
				t.Fatal(err)
			}
			// Go randomizes map iteration per range, so a handful of repeats
			// reliably catches an order-dependent emitter.
			for i := range 50 {
				again, err := htmlbind.Generate(path, source, htmlbind.GenerateOptions{})
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(first, again) {
					t.Fatalf("generation %d differs from the first:\n--- first ---\n%s--- again ---\n%s", i, first, again)
				}
			}
		})
	}
}
