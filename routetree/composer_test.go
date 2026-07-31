package routetree

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func compose(t *testing.T, route Route, layouts []ComponentSignature) string {
	t.Helper()
	source, err := NewEmitter().Composer(route, layouts)
	if err != nil {
		t.Fatalf("Composer: %v", err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "gen.go", source, parser.SkipObjectResolution); err != nil {
		t.Fatalf("generated source does not parse: %v\n%s", err, source)
	}
	return string(source)
}

func slotOnly(name string) ComponentSignature {
	return ComponentSignature{Name: name, Slots: []Value{{Name: SlotParamName, Type: "htmlbind.Fragment"}}}
}

func TestComposerWithoutLayoutsRendersAlone(t *testing.T) {
	route := Route{Path: "/about", Package: "about", PageFile: "app/about/page.tb.html"}
	source := compose(t, route, nil)

	mustContain(t, source,
		"package about",
		"func Render(w io.Writer, route RouteParams, params PageParams, options ...htmlbind.Option) error",
		"htmlbind.Render(w, Page(params), options...)",
	)
	if strings.Contains(source, "RenderChain") {
		t.Errorf("no layouts but a chain was emitted:\n%s", source)
	}
}

func TestComposerWrapsAncestorLayoutsOutermostFirst(t *testing.T) {
	route := Route{
		Path:       "/users/{id}",
		RelDir:     "users/id_",
		Package:    "id_",
		PageFile:   "app/users/id_/page.tb.html",
		ImportPath: "example.com/m/app/users/id_",
		Params:     []Segment{dyn("id")},
		Layouts: []Layout{
			{RelDir: "", Package: "app", ImportPath: "example.com/m/app", File: "app/layout.tb.html"},
			{RelDir: "users", Package: "users", ImportPath: "example.com/m/app/users", File: "app/users/layout.tb.html"},
		},
	}
	source := compose(t, route, []ComponentSignature{slotOnly("Layout"), slotOnly("Layout")})

	mustContain(t, source,
		`"example.com/m/app"`,
		`"example.com/m/app/users"`,
		"wrappers := []htmlbind.Wrapper{",
		"app.BindLayout(app.LayoutParams{",
		"users.BindLayout(users.LayoutParams{",
		"htmlbind.RenderChain(w, wrappers, Page(params), options...)",
	)
	// Outermost first is the chain contract, so the root layout must precede.
	if strings.Index(source, "app.BindLayout") > strings.Index(source, "users.BindLayout") {
		t.Errorf("layouts emitted innermost first:\n%s", source)
	}
}

func TestComposerFillsScopedLayoutParameters(t *testing.T) {
	route := Route{
		Path:     "/users/{id}/posts",
		RelDir:   "users/id_/posts",
		Package:  "posts",
		PageFile: "app/users/id_/posts/page.tb.html",
		Params:   []Segment{dyn("id")},
		Layouts: []Layout{
			{
				RelDir: "users/id_", Package: "id_",
				ImportPath: "example.com/m/app/users/id_",
				File:       "app/users/id_/layout.tb.html",
				Params:     []Segment{dyn("id")},
			},
		},
	}
	layout := slotOnly("Layout")
	layout.Inputs = []Value{{Name: "id", Type: "string"}}

	source := compose(t, route, []ComponentSignature{layout})
	mustContain(t, source, "ID: route.ID,")
}

func TestComposerRejectsALayoutReadingADeeperSegment(t *testing.T) {
	// The users layout encloses no dynamic segment, so it cannot read id.
	route := Route{
		Path:     "/users/{id}",
		Package:  "id_",
		PageFile: "app/users/id_/page.tb.html",
		Params:   []Segment{dyn("id")},
		Layouts: []Layout{
			{RelDir: "users", Package: "users", ImportPath: "example.com/m/app/users", File: "app/users/layout.tb.html"},
		},
	}
	layout := slotOnly("Layout")
	layout.Inputs = []Value{{Name: "id", Type: "string"}}

	_, err := NewEmitter().Composer(route, []ComponentSignature{layout})
	if err == nil {
		t.Fatal("out-of-scope layout parameter accepted, want rejection")
	}
	if !strings.Contains(err.Error(), "only read segments it encloses") {
		t.Errorf("error = %v, want it to state the scope rule", err)
	}
}

func TestComposerRejectsALayoutWithoutASlot(t *testing.T) {
	route := Route{
		Path:     "/",
		Package:  "app",
		PageFile: "app/page.tb.html",
		Layouts:  []Layout{{RelDir: "", Package: "app", File: "app/layout.tb.html"}},
	}
	_, err := NewEmitter().Composer(route, []ComponentSignature{{Name: "Layout"}})
	if err == nil {
		t.Fatal("slotless layout accepted, want rejection")
	}
	if !strings.Contains(err.Error(), "children: html") {
		t.Errorf("error = %v, want it to state the required declaration", err)
	}
}

func TestComposerRejectsMismatchedSignatureCount(t *testing.T) {
	route := Route{
		Path:     "/",
		Package:  "app",
		PageFile: "app/page.tb.html",
		Layouts:  []Layout{{RelDir: "", Package: "app", File: "app/layout.tb.html"}},
	}
	if _, err := NewEmitter().Composer(route, nil); err == nil {
		t.Fatal("missing signature accepted, want rejection")
	}
}

func TestComposerSkipsTheSelectorForItsOwnPackage(t *testing.T) {
	// A layout beside the page is in the same Go package, so its symbols need
	// no qualifier and no import.
	route := Route{
		Path:       "/settings",
		RelDir:     "settings",
		Package:    "settings",
		PageFile:   "app/settings/page.tb.html",
		ImportPath: "example.com/m/app/settings",
		Layouts: []Layout{
			{RelDir: "settings", Package: "settings", ImportPath: "example.com/m/app/settings", File: "app/settings/layout.tb.html"},
		},
	}
	source := compose(t, route, []ComponentSignature{slotOnly("Layout")})

	mustContain(t, source, "BindLayout(LayoutParams{")
	if strings.Contains(source, "settings.BindLayout") {
		t.Errorf("own package was qualified:\n%s", source)
	}
	if strings.Contains(source, `"example.com/m/app/settings"`) {
		t.Errorf("own package was imported:\n%s", source)
	}
}

func TestComposerDisambiguatesCollidingPackageNames(t *testing.T) {
	// Two ancestors named the same on disk must not produce one selector.
	route := Route{
		Path:     "/a/b",
		RelDir:   "a/b",
		Package:  "b",
		PageFile: "app/a/b/page.tb.html",
		Layouts: []Layout{
			{RelDir: "a", Package: "shared", ImportPath: "example.com/m/app/a", File: "app/a/layout.tb.html"},
			{RelDir: "a/x", Package: "shared", ImportPath: "example.com/m/app/a/x", File: "app/a/x/layout.tb.html"},
		},
	}
	source := compose(t, route, []ComponentSignature{slotOnly("Layout"), slotOnly("Layout")})

	mustContain(t, source, "shared.BindLayout", "shared1.BindLayout")
	if !strings.Contains(source, `shared1 "example.com/m/app/a/x"`) {
		t.Errorf("disambiguated import not aliased:\n%s", source)
	}
}

func TestComposerHonorsRenamedDeclarations(t *testing.T) {
	e := NewEmitter()
	e.RenderFunc = "Compose"
	e.ParamsType = "PageInput"

	source, err := e.Composer(Route{Path: "/x", Package: "x", PageFile: "app/x/page.tb.html"}, nil)
	if err != nil {
		t.Fatalf("Composer: %v", err)
	}
	mustContain(t, string(source), "func Compose(w io.Writer, route PageInput,")
}

func TestComposerRepointsTheRuntime(t *testing.T) {
	e := NewEmitter()
	e.Symbols.RuntimeImport = "example.com/fw/render"
	e.Symbols.RuntimeAlias = "render"

	source, err := e.Composer(Route{Path: "/x", Package: "x", PageFile: "app/x/page.tb.html"}, nil)
	if err != nil {
		t.Fatalf("Composer: %v", err)
	}
	got := string(source)
	mustContain(t, got, `"example.com/fw/render"`, "render.Render(w, Page(params), options...)")
	if strings.Contains(got, "htmlbind.") {
		t.Errorf("default runtime still referenced:\n%s", got)
	}
}

// The framework entry a render override targets: a writer, a request, the chain,
// and the leaf, which is the shape Symbols alone could never reach.
const frameworkRenderBlock = `web.WriteHTML({{ .Writer }}, {{ .Request }}, {{ .Chain }}, {{ .Leaf }})`

func TestComposerRenderBlockIsReplaceable(t *testing.T) {
	e := NewEmitter()
	e.RenderRequestParam = "r"
	if err := e.Parse(TemplateRender, frameworkRenderBlock); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	route := Route{
		Path:     "/",
		Package:  "app",
		PageFile: "app/page.tb.html",
		Layouts:  []Layout{{RelDir: "", Package: "app", File: "app/layout.tb.html"}},
	}
	source, err := e.Composer(route, []ComponentSignature{slotOnly("Layout")})
	if err != nil {
		t.Fatalf("Composer: %v", err)
	}
	got := string(source)
	mustContain(t, got,
		"func Render(w io.Writer, r *http.Request, route RouteParams,",
		"return web.WriteHTML(w, r, wrappers, Page(params))",
		`"net/http"`,
	)
	if strings.Contains(got, "htmlbind.RenderChain") {
		t.Errorf("the default render call survived the override:\n%s", got)
	}
}

func TestComposerWithoutARequestParameterLeavesItOutOfTheBlock(t *testing.T) {
	e := NewEmitter()
	if err := e.Parse(TemplateRender, `web.WriteHTML({{ .Writer }}{{ with .Request }}, {{ . }}{{ end }}, {{ .Leaf }})`); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	source, err := e.Composer(Route{Path: "/x", Package: "x", PageFile: "app/x/page.tb.html"}, nil)
	if err != nil {
		t.Fatalf("Composer: %v", err)
	}
	got := string(source)
	// The default composer takes a writer only, so an override must be able to
	// see that no request is in scope rather than emitting a name that is not.
	mustContain(t, got, "return web.WriteHTML(w, Page(params))")
	if strings.Contains(got, `"net/http"`) {
		t.Errorf("request package imported without a request parameter:\n%s", got)
	}
}

func TestComposerTakesAConfiguredWriterType(t *testing.T) {
	e := NewEmitter()
	e.RenderWriterType = "http.ResponseWriter"

	source, err := e.Composer(Route{Path: "/x", Package: "x", PageFile: "app/x/page.tb.html"}, nil)
	if err != nil {
		t.Fatalf("Composer: %v", err)
	}
	got := string(source)
	mustContain(t, got, "func Render(w http.ResponseWriter, route RouteParams,", `"net/http"`)
	// The writer no longer comes from io, so importing it would not compile.
	if strings.Contains(got, `"io"`) {
		t.Errorf("unused io import emitted:\n%s", got)
	}
}

func TestComposerWriterTypeFromTheRuntimeNeedsNoExtraImport(t *testing.T) {
	e := NewEmitter()
	e.Symbols.RuntimeImport = "example.com/fw/render"
	e.Symbols.RuntimeAlias = "render"
	e.RenderWriterType = "render.Response"

	source, err := e.Composer(Route{Path: "/x", Package: "x", PageFile: "app/x/page.tb.html"}, nil)
	if err != nil {
		t.Fatalf("Composer: %v", err)
	}
	got := string(source)
	mustContain(t, got, `import "example.com/fw/render"`, "func Render(w render.Response, route RouteParams,")
	if strings.Contains(got, `"io"`) || strings.Contains(got, `"net/http"`) {
		t.Errorf("unused import emitted:\n%s", got)
	}
}

func TestComposerTemplateIsReplaceable(t *testing.T) {
	e := NewEmitter()
	if err := e.Parse(TemplateComposer, `{{ .Header }}

package {{ .Package }}

// {{ .Pattern }} wraps {{ len .Layouts }} layout(s).
const Layouts = {{ len .Layouts }}
`); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	route := Route{
		Path:     "/",
		Package:  "app",
		PageFile: "app/page.tb.html",
		Layouts:  []Layout{{RelDir: "", Package: "app", File: "app/layout.tb.html"}},
	}
	source, err := e.Composer(route, []ComponentSignature{slotOnly("Layout")})
	if err != nil {
		t.Fatalf("Composer: %v", err)
	}
	mustContain(t, string(source), "const Layouts = 1")
}

func TestComposerIsDeterministic(t *testing.T) {
	route := Route{
		Path:     "/a/b",
		Package:  "b",
		PageFile: "app/a/b/page.tb.html",
		Layouts: []Layout{
			{RelDir: "a", Package: "a", ImportPath: "example.com/m/app/a", File: "app/a/layout.tb.html"},
			{RelDir: "a/b", Package: "b2", ImportPath: "example.com/m/app/a/b2", File: "app/a/b2/layout.tb.html"},
		},
	}
	sigs := []ComponentSignature{slotOnly("Layout"), slotOnly("Layout")}
	first := compose(t, route, sigs)
	for range 5 {
		if got := compose(t, route, sigs); got != first {
			t.Fatal("Composer is not deterministic")
		}
	}
}
