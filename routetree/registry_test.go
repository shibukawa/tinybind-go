package routetree

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func registry(t *testing.T, e *Emitter, tree *Tree, analyses []Analysis, layouts map[string]ComponentSignature) string {
	t.Helper()
	if e == nil {
		e = NewEmitter()
	}
	source, err := e.Registry(tree, "pages", analyses, layouts, nil)
	if err != nil {
		t.Fatalf("Registry: %v", err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "gen.go", source, parser.SkipObjectResolution); err != nil {
		t.Fatalf("generated source does not parse: %v\n%s", err, source)
	}
	return string(source)
}

// templateOnly builds a route plus the analysis a template-only page produces.
func templateOnly(path, relDir, pkg, importPath string, params []Segment, inputs []Value, layouts ...Layout) (Route, Analysis) {
	route := Route{
		Path: path, RelDir: relDir, Package: pkg, ImportPath: importPath,
		PageFile: "pages/" + relDir + "/page.tb.html",
		Params:   params, Layouts: layouts,
	}
	return route, Analysis{
		Route:     route,
		Component: ComponentSignature{Name: "Page", Inputs: inputs},
		Page:      &PageFunc{Rung: RungTemplateOnly},
		Inputs:    inputs,
	}
}

func TestRegistryRegistersEveryRoute(t *testing.T) {
	home, homeAnalysis := templateOnly("/", "", "pages", "example.com/m/pages", nil, nil)
	about, aboutAnalysis := templateOnly("/about", "about", "about", "example.com/m/pages/about", nil,
		[]Value{{Name: "topic", Type: "string"}})

	source := registry(t, nil, &Tree{Routes: []Route{home, about}},
		[]Analysis{homeAnalysis, aboutAnalysis}, nil)

	mustContain(t, source,
		"package pages",
		"func Register(mux *http.ServeMux, options ...htmlbind.Option)",
		"func NewServeMux(options ...htmlbind.Option) *http.ServeMux",
		`mux.HandleFunc("GET /{$}"`,
		`mux.HandleFunc("GET /about"`,
		"about.PageParams{",
		"Topic: route.Topic,",
	)
	// The root page lives in the registry's own package, so it needs no
	// qualifier and no import.
	if strings.Contains(source, "pages.Page(") {
		t.Errorf("root package was qualified:\n%s", source)
	}
}

func TestRegistryRootRegistersAsAnExactMatch(t *testing.T) {
	home, analysis := templateOnly("/", "", "pages", "example.com/m/pages", nil, nil)
	source := registry(t, nil, &Tree{Routes: []Route{home}}, []Analysis{analysis}, nil)

	if !strings.Contains(source, `"GET /{$}"`) {
		t.Errorf("root registered as a prefix pattern, which would answer every 404:\n%s", source)
	}
}

func TestRegistryCallsATypedPageBeforeRendering(t *testing.T) {
	route := Route{
		Path: "/users/{id}", RelDir: "users/id_", Package: "id_",
		ImportPath: "example.com/m/pages/users/id_",
		PageFile:   "pages/users/id_/page.tb.html",
		Params:     []Segment{dyn("id")},
	}
	analysis := Analysis{
		Route:     route,
		Component: ComponentSignature{Name: "Page", Inputs: []Value{{Name: "name", Type: "string"}}},
		Page: &PageFunc{
			Rung:    RungTypedPage,
			Params:  []Value{{Name: "id", Type: "string"}},
			Results: []Value{{Type: "string"}},
		},
		Inputs: []Value{{Name: "id", Type: "string"}},
	}

	source := registry(t, nil, &Tree{Routes: []Route{route}}, []Analysis{analysis}, nil)
	mustContain(t, source,
		"pageName, err := id_.Load(route.ID)",
		"Name: pageName,",
		"id_.Page(params)",
	)
}

func TestRegistryRegistersARawHandlerDirectly(t *testing.T) {
	route := Route{
		Path: "/stream", RelDir: "stream", Package: "stream",
		ImportPath: "example.com/m/pages/stream",
		PageFile:   "pages/stream/page.tb.html",
	}
	analysis := Analysis{Route: route, Page: &PageFunc{Rung: RungHandlerPage}}

	source := registry(t, nil, &Tree{Routes: []Route{route}}, []Analysis{analysis}, nil)
	mustContain(t, source, `mux.HandleFunc("GET /stream", stream.Load)`)
	// A raw handler owns its whole response, so the registry adds no body.
	if strings.Contains(source, "stream.DecodeRoute") {
		t.Errorf("registry decoded for a raw handler:\n%s", source)
	}
}

func TestRegistryWrapsLayouts(t *testing.T) {
	layout := Layout{RelDir: "", Package: "pages", ImportPath: "example.com/m/pages", File: "pages/layout.tb.html"}
	about, analysis := templateOnly("/about", "about", "about", "example.com/m/pages/about", nil, nil, layout)

	source := registry(t, nil, &Tree{Routes: []Route{about}}, []Analysis{analysis},
		map[string]ComponentSignature{"": slotOnly("Layout")})

	mustContain(t, source,
		"wrappers := []htmlbind.Wrapper{",
		"BindLayout(LayoutParams{",
		"htmlbind.RenderChain(w, wrappers, about.Page(params), options...)",
	)
}

func TestRegistryEmitsTheRouteTable(t *testing.T) {
	users, analysis := templateOnly("/users/{id}", "users/id_", "id_", "example.com/m/pages/users/id_",
		[]Segment{dyn("id")}, []Value{{Name: "id", Type: "string"}})

	source := registry(t, nil, &Tree{Routes: []Route{users}}, []Analysis{analysis}, nil)
	mustContain(t, source,
		"var Routes = []RouteInfo{",
		`{Pattern: "GET /users/{id}", Path: "/users/{id}", Dir: "users/id_", Params: []string{"id"}}`,
		"type RouteInfo struct",
	)
}

func TestRegistryRejectsAMissingLayoutSignature(t *testing.T) {
	layout := Layout{RelDir: "", Package: "pages", File: "pages/layout.tb.html"}
	about, analysis := templateOnly("/about", "about", "about", "example.com/m/pages/about", nil, nil, layout)

	_, err := NewEmitter().Registry(&Tree{Routes: []Route{about}}, "pages", []Analysis{analysis}, nil, nil)
	if err == nil {
		t.Fatal("missing layout signature accepted, want rejection")
	}
	if !strings.Contains(err.Error(), "no signature supplied") {
		t.Errorf("error = %v", err)
	}
}

func TestRegistryRejectsASlotlessLayout(t *testing.T) {
	layout := Layout{RelDir: "", Package: "pages", File: "pages/layout.tb.html"}
	about, analysis := templateOnly("/about", "about", "about", "example.com/m/pages/about", nil, nil, layout)

	_, err := NewEmitter().Registry(&Tree{Routes: []Route{about}}, "pages", []Analysis{analysis},
		map[string]ComponentSignature{"": {Name: "Layout"}}, nil)
	if err == nil {
		t.Fatal("slotless layout accepted, want rejection")
	}
	if !strings.Contains(err.Error(), "children: html") {
		t.Errorf("error = %v, want it to state the required declaration", err)
	}
}

func TestRegistryRejectsMismatchedAnalysisCount(t *testing.T) {
	home, _ := templateOnly("/", "", "pages", "example.com/m/pages", nil, nil)
	if _, err := NewEmitter().Registry(&Tree{Routes: []Route{home}}, "pages", nil, nil, nil); err == nil {
		t.Fatal("missing analysis accepted, want rejection")
	}
}

func TestRegistryRejectsATypedPageWhoseResultsDoNotFitTheComponent(t *testing.T) {
	route := Route{Path: "/x", RelDir: "x", Package: "x", ImportPath: "example.com/m/pages/x", PageFile: "pages/x/page.tb.html"}
	analysis := Analysis{
		Route:     route,
		Component: ComponentSignature{Name: "Page", Inputs: []Value{{Name: "a", Type: "string"}, {Name: "b", Type: "int"}}},
		Page:      &PageFunc{Rung: RungTypedPage, File: "pages/x/page.go", Results: []Value{{Type: "string"}}},
	}
	_, err := NewEmitter().Registry(&Tree{Routes: []Route{route}}, "pages", []Analysis{analysis}, nil, nil)
	if err == nil {
		t.Fatal("arity mismatch accepted, want rejection")
	}
	if !strings.Contains(err.Error(), "2 parameter") {
		t.Errorf("error = %v", err)
	}
}

func TestRegistryHonorsRenamedDeclarations(t *testing.T) {
	e := NewEmitter()
	e.RegisterFunc = "Install"
	e.MuxFunc = "Mux"
	e.TableVar = "Table"
	e.DecodeFunc = "Bind"

	about, analysis := templateOnly("/about", "about", "about", "example.com/m/pages/about", nil,
		[]Value{{Name: "topic", Type: "string"}})
	source := registry(t, e, &Tree{Routes: []Route{about}}, []Analysis{analysis}, nil)

	mustContain(t, source, "func Install(mux", "func Mux(", "var Table = []RouteInfo{", "about.Bind(r)")
}

func TestRegistryRepointsSymbols(t *testing.T) {
	e := NewEmitter()
	e.Symbols.RuntimeImport = "example.com/fw/render"
	e.Symbols.RuntimeAlias = "render"
	e.Symbols.ErrorImport = "example.com/fw/web"
	e.Symbols.ErrorAlias = "web"
	// The failure entry is a name like the error constructors beside it, so a
	// framework spelling it WriteProblem needs no template of its own.
	e.Symbols.WriteError = "WriteProblem"

	home, analysis := templateOnly("/", "", "pages", "example.com/m/pages", nil, nil)
	source := registry(t, e, &Tree{Routes: []Route{home}}, []Analysis{analysis}, nil)

	mustContain(t, source, "render.Render(w,", "web.WriteProblem(w, r, err)")
	if strings.Contains(source, "htmlbind.") || strings.Contains(source, "httpbind.") {
		t.Errorf("default runtime still referenced:\n%s", source)
	}
}

func TestRegistryRenderBlockReceivesTheRequest(t *testing.T) {
	e := NewEmitter()
	if err := e.Parse(TemplateRender, frameworkRenderBlock); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	layout := Layout{RelDir: "", Package: "pages", ImportPath: "example.com/m/pages", File: "pages/layout.tb.html"}
	about, analysis := templateOnly("/about", "about", "about", "example.com/m/pages/about", nil, nil, layout)

	source := registry(t, e, &Tree{Routes: []Route{about}}, []Analysis{analysis},
		map[string]ComponentSignature{"": slotOnly("Layout")})

	// The request is already in scope in a generated handler, so an override
	// reaches it with no setting at all. This is the whole reason the block
	// exists: the error entry took (w, r) and the render entry could not.
	mustContain(t, source,
		"if err := web.WriteHTML(w, r, wrappers, about.Page(params)); err != nil {",
		"httpbind.WriteError(w, r, err)",
	)
	if strings.Contains(source, "htmlbind.RenderChain") {
		t.Errorf("the default render call survived the override:\n%s", source)
	}
}

func TestRegistryRenderBlockSeesNoChainWithoutLayouts(t *testing.T) {
	about, analysis := templateOnly("/about", "about", "about", "example.com/m/pages/about", nil, nil)
	tree := &Tree{Routes: []Route{about}}

	// An entry that always takes a chain reads Chain, which is nil for a page
	// with no ancestor layout, so the override needs no branch of its own.
	chained := NewEmitter()
	if err := chained.Parse(TemplateRender, frameworkRenderBlock); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	mustContain(t, registry(t, chained, tree, []Analysis{analysis}, nil),
		"web.WriteHTML(w, r, nil, about.Page(params))")

	// An override that would rather branch reads Wrappers, which is empty here.
	branching := NewEmitter()
	if err := branching.Parse(TemplateRender, `web.WriteHTML({{ .Writer }}, {{ .Request }}{{ with .Wrappers }}, {{ . }}{{ end }}, {{ .Leaf }})`); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	mustContain(t, registry(t, branching, tree, []Analysis{analysis}, nil),
		"web.WriteHTML(w, r, about.Page(params))")
}

func TestRegistryTakesAFrameworkRouter(t *testing.T) {
	e := NewEmitter()
	e.Symbols.MuxImport = "example.com/fw/web"
	e.Symbols.MuxAlias = "web"
	e.Symbols.MuxType = "web.Router"
	e.Symbols.MuxConstructor = "web.NewRouter"

	about, analysis := templateOnly("/about", "about", "about", "example.com/m/pages/about", nil, nil)
	source := registry(t, e, &Tree{Routes: []Route{about}}, []Analysis{analysis}, nil)

	mustContain(t, source,
		"func Register(mux web.Router,",
		"func NewServeMux(options ...htmlbind.Option) web.Router",
		"mux := web.NewRouter()",
		`"example.com/fw/web"`,
		// The handler body still declares the stdlib request pair, so moving the
		// router does not move the request package with it.
		`"net/http"`,
		"func(w http.ResponseWriter, r *http.Request)",
	)
}

func TestRegistryOmitsTheRequestImportWhenNoHandlerIsGenerated(t *testing.T) {
	e := NewEmitter()
	e.Symbols.MuxImport = "example.com/fw/web"
	e.Symbols.MuxAlias = "web"
	e.Symbols.MuxType = "web.Router"
	e.Symbols.MuxConstructor = "web.NewRouter"

	route := Route{
		Path: "/stream", RelDir: "stream", Package: "stream",
		ImportPath: "example.com/m/pages/stream",
		PageFile:   "pages/stream/page.tb.html",
	}
	analysis := Analysis{Route: route, Page: &PageFunc{Rung: RungHandlerPage}}

	source := registry(t, e, &Tree{Routes: []Route{route}}, []Analysis{analysis}, nil)
	// A raw handler owns its response, so nothing in this registry names Request;
	// importing it anyway would not compile.
	if strings.Contains(source, `"net/http"`) {
		t.Errorf("unused request import emitted:\n%s", source)
	}
}

func TestRegistryOmitsTheConstructorWithoutOne(t *testing.T) {
	e := NewEmitter()
	e.Symbols.MuxType = "web.Router"
	e.Symbols.MuxConstructor = ""
	e.Symbols.MuxImport = "example.com/fw/web"
	e.Symbols.MuxAlias = "web"

	about, analysis := templateOnly("/about", "about", "about", "example.com/m/pages/about", nil, nil)
	source := registry(t, e, &Tree{Routes: []Route{about}}, []Analysis{analysis}, nil)

	mustContain(t, source, "func Register(mux web.Router,")
	// A router needing arguments cannot be built by generated code, so the
	// constructor is left out rather than emitted broken.
	if strings.Contains(source, "func NewServeMux(") {
		t.Errorf("constructor emitted without a constructor symbol:\n%s", source)
	}
}

func TestRegistryTemplateIsReplaceable(t *testing.T) {
	e := NewEmitter()
	if err := e.Parse(TemplateRegistry, `{{ .Header }}

package {{ .Package }}

const RouteCount = {{ len .Routes }}
`); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	home, analysis := templateOnly("/", "", "pages", "example.com/m/pages", nil, nil)
	source := registry(t, e, &Tree{Routes: []Route{home}}, []Analysis{analysis}, nil)
	mustContain(t, source, "const RouteCount = 1")
}

func TestRegistryIsDeterministic(t *testing.T) {
	home, homeAnalysis := templateOnly("/", "", "pages", "example.com/m/pages", nil, nil)
	about, aboutAnalysis := templateOnly("/about", "about", "about", "example.com/m/pages/about", nil, nil)
	users, usersAnalysis := templateOnly("/users/{id}", "users/id_", "id_", "example.com/m/pages/users/id_",
		[]Segment{dyn("id")}, []Value{{Name: "id", Type: "string"}})

	tree := &Tree{Routes: []Route{home, about, users}}
	analyses := []Analysis{homeAnalysis, aboutAnalysis, usersAnalysis}
	first := registry(t, nil, tree, analyses, nil)
	for range 5 {
		if got := registry(t, nil, tree, analyses, nil); got != first {
			t.Fatal("Registry is not deterministic")
		}
	}
}
