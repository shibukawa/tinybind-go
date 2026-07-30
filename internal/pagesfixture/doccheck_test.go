package pagesfixture

import (
	"errors"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
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

// TestDocCustomizationExamplesCompile covers the snippets the guide publishes for
// the seams a framework reaches for when a symbol rename is not enough.
func TestDocCustomizationExamplesCompile(t *testing.T) {
	e := routetree.NewEmitter()
	e.Symbols.MuxImport = "example.com/framework/web"
	e.Symbols.MuxAlias = "web"
	e.Symbols.MuxType = "web.Router"
	e.Symbols.MuxConstructor = "web.NewRouter"
	e.RenderWriterType = "http.ResponseWriter"
	e.RenderRequestParam = "r"

	if err := e.Parse(routetree.TemplateRender,
		`web.WriteHTML({{ .Writer }}, {{ .Request }}, {{ .Chain }}, {{ .Leaf }})`); err != nil {
		t.Fatalf("documented render override failed: %v", err)
	}

	route := routetree.Route{Path: "/", Package: "pages", PageFile: "pages/page.tb.html"}
	source, err := e.Composer(route, nil)
	if err != nil {
		t.Fatalf("documented composer settings failed: %v", err)
	}
	if !strings.Contains(string(source), "func Render(w http.ResponseWriter, r *http.Request, route RouteParams,") {
		t.Errorf("composer entry does not match the documented shape:\n%s", source)
	}
}

// TestDocActionResolverExampleCompiles covers the snippet for a template naming a
// handler the tree does not hold.
func TestDocActionResolverExampleCompiles(t *testing.T) {
	myRouteTable := map[string]string{"Publish": "/app/publish"}
	files, err := routetree.Generate(routetree.GenerateOptions{
		Config: routetree.Config{
			Root:       "pages",
			ImportBase: importBase,
		},
		RootPackage: "pages",
		ActionResolver: func(name string) (string, bool) {
			url, ok := myRouteTable[name]
			return url, ok
		},
	})
	if err != nil {
		t.Fatalf("documented ActionResolver example failed: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("documented ActionResolver example produced nothing")
	}
}

// TestDocPackagesLoopExampleCompiles covers the loop the guide publishes for
// generating a binder per route package.
func TestDocPackagesLoopExampleCompiles(t *testing.T) {
	tree, err := routetree.Discover(routetree.Config{Root: "pages", ImportBase: importBase})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	packages := tree.Packages()
	if len(packages) < 2 {
		t.Fatalf("Packages = %+v, want the root and every route directory", packages)
	}
	if packages[0].RelDir != "" || packages[0].Dir != tree.Root {
		t.Errorf("Packages[0] = %+v, want the route root first", packages[0])
	}
	for _, pkg := range packages {
		if pkg.Name == "" || pkg.ImportPath == "" {
			t.Errorf("package %+v is not addressable by the generator", pkg)
		}
	}

	// The documented loop tolerates one error and no other, so the sentinel it
	// names has to be what a package with nothing to bind actually returns.
	skipped, bound := 0, 0
	for _, pkg := range packages {
		_, err := generator.Generate(pkg.Dir, t.TempDir(), "tinybind_gen.go")
		switch {
		case err == nil:
			bound++
		case errors.Is(err, generator.ErrNothingToGenerate):
			skipped++
		default:
			t.Errorf("%s: %v", pkg.RelDir, err)
		}
	}
	if bound == 0 || skipped == 0 {
		t.Errorf("bound=%d skipped=%d, want the loop to exercise both outcomes", bound, skipped)
	}
}
