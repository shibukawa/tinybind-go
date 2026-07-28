package pagesfixture

import (
	"testing"

	"github.com/shibukawa/tinybind-go/routetree"
)

// TestDocExampleCompiles keeps the framework-owner guide honest: the snippet it
// publishes is the snippet compiled here.
func TestDocExampleCompiles(t *testing.T) {
	e := routetree.NewEmitter()
	e.Symbols.RuntimeImport = "example.com/framework/render"
	e.Symbols.RuntimeAlias = "render"
	e.Symbols.ErrorImport = "example.com/framework/web"
	e.Symbols.ErrorAlias = "web"
	e.Symbols.BadRequest = "Invalid"
	e.Symbols.Problem = "Fault"

	if err := e.Parse("error", `web.Invalid(web.Fault{Code: {{ .Code | quote }}})`); err != nil {
		t.Fatalf("documented Parse example failed: %v", err)
	}
	if _, err := e.Clone(); err != nil {
		t.Fatalf("documented Clone example failed: %v", err)
	}

	files, err := routetree.Generate(routetree.GenerateOptions{
		Config: routetree.Config{
			Root:       "pages",
			ImportBase: "github.com/shibukawa/tinybind-go/internal/pagesfixture/pages",
		},
		RootPackage: "pages",
	})
	if err != nil {
		t.Fatalf("documented Generate example failed: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("documented Generate example produced nothing")
	}
}
