package routetree_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/routetree"
)

// The typed rung is gone. A page.go declaring a Load that is not the handler
// shape is refused rather than accepted into a contract that no longer exists,
// and the message says what to write instead — otherwise the only clue would be
// that a signature which used to work now does not.
func TestANonHandlerLoadIsRefused(t *testing.T) {
	dir := t.TempDir()
	pages := filepath.Join(dir, "pages")
	if err := os.MkdirAll(pages, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(pages, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("page.tb.html", "export component Page(): html {\n<p>hi</p>\n}\n")
	write("page.go", "package pages\n\nfunc Load() (string, error) { return \"\", nil }\n")

	_, err := routetree.Generate(routetree.GenerateOptions{
		Config:      routetree.Config{Root: pages, ImportBase: "example.com/m/pages"},
		RootPackage: "pages",
	})
	if err == nil {
		t.Fatal("a typed Load was accepted, want a refusal")
	}
	for _, want := range []string{"must be func(http.ResponseWriter, *http.Request)", "binds it with {val}"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %v, want it to mention %q", err, want)
		}
	}
}
