package routetree

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fastDecoder(t *testing.T, route Route, inputs []Value) string {
	t.Helper()
	source, err := NewFastHTTPEmitter("").Decoder(route, inputs)
	if err != nil {
		t.Fatalf("Decoder: %v", err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "gen.go", source, parser.SkipObjectResolution); err != nil {
		t.Fatalf("generated source does not parse: %v\n%s", err, source)
	}
	return string(source)
}

// TestFastHTTPDecoderReadsThroughTheRuntime is the substitution the port turns
// on: the decoder body is the same statements on either transport, because the
// accessors carry one name and take the transport first.
func TestFastHTTPDecoderReadsThroughTheRuntime(t *testing.T) {
	route := decoderRoute("/users/{id}", "id_", dyn("id"))
	source := fastDecoder(t, route, []Value{
		{Name: "id", Type: "string"},
		{Name: "page", Type: "int"},
	})

	mustContain(t, source,
		"func DecodeRoute(ctx *fasthttp.RequestCtx) (RouteParams, error)",
		`httpbind.PathValue(ctx, "id")`,
		"query := httpbind.Queries(ctx)",
		`httpbind.QueryLookup(query, "page")`,
		`"github.com/shibukawa/tinygodriver/fasthttp"`,
		`httpbind "github.com/shibukawa/tinybind-go/fasthttpbind"`,
	)
	if strings.Contains(source, "net/http") {
		t.Errorf("a fasthttp decoder names net/http:\n%s", source)
	}
}

// TestFastHTTPDecoderTakesANamedTransportPackage keeps this module out of the
// business of deciding which fasthttp an application depends on.
func TestFastHTTPDecoderTakesANamedTransportPackage(t *testing.T) {
	e := NewFastHTTPEmitter("github.com/valyala/fasthttp")
	source, err := e.Decoder(decoderRoute("/files/{rest...}", "rest__", catchAll("rest")),
		[]Value{{Name: "rest", Type: "string"}})
	if err != nil {
		t.Fatalf("Decoder: %v", err)
	}
	mustContain(t, string(source), `"github.com/valyala/fasthttp"`, "ctx *fasthttp.RequestCtx")
}

// TestFastHTTPRegistryEmitsTheCollapsedHandler covers the half no alias could
// reach: the handler literal has one parameter rather than two, and the runtime
// call that took both takes one.
func TestFastHTTPRegistryEmitsTheCollapsedHandler(t *testing.T) {
	home, homeAnalysis := templateOnly("/", "", "pages", "example.com/m/pages", nil, nil)
	source := registry(t, NewFastHTTPEmitter(""), &Tree{Routes: []Route{home}}, []Analysis{homeAnalysis}, nil)

	mustContain(t, source,
		"func Register(mux interface { HandleFunc(string, func(*fasthttp.RequestCtx)) }, options ...htmlbind.Option)",
		"func(ctx *fasthttp.RequestCtx) {",
		"DecodeRoute(ctx)",
		"httpbind.WriteError(ctx, err)",
		"htmlbind.WithContext(ctx)",
	)
	// fasthttp ships no router, so there is nothing for a constructor to build.
	if strings.Contains(source, "func NewServeMux") {
		t.Errorf("a router constructor was emitted for a transport with no router:\n%s", source)
	}
}

// TestTransportArgsCollapse states the rule the registry template relies on: a
// call taking the writer and the request takes one value where one carries
// both, and two where they are distinct.
func TestTransportArgsCollapse(t *testing.T) {
	if got := DefaultSymbols().TransportArgs(); got != "w, r" {
		t.Errorf("net/http TransportArgs = %q, want %q", got, "w, r")
	}
	if got := FastHTTPSymbols("").TransportArgs(); got != "ctx" {
		t.Errorf("fasthttp TransportArgs = %q, want %q", got, "ctx")
	}
}

// TestContextOfReadsTheTransportsOwnSpelling covers the difference the rewrite
// table names as a selector: one transport reads a context off the request with
// a call, and the other's request value is one.
func TestContextOfReadsTheTransportsOwnSpelling(t *testing.T) {
	if got := DefaultSymbols().ContextOf("r"); got != "r.Context()" {
		t.Errorf("net/http ContextOf = %q", got)
	}
	if got := FastHTTPSymbols("").ContextOf("ctx"); got != "ctx" {
		t.Errorf("fasthttp ContextOf = %q", got)
	}
	// The composer's request parameter is named by the caller, so the identifier
	// is an argument rather than read off Symbols.
	if got := DefaultSymbols().ContextOf("req"); got != "req.Context()" {
		t.Errorf("ContextOf(req) = %q", got)
	}
	if got := DefaultSymbols().ContextOf(""); got != "" {
		t.Errorf("ContextOf with no request in scope = %q, want empty", got)
	}
}

// TestEmptyTransportFieldsKeepTheNetHTTPShape is the compatibility half: an
// emitter configured before these fields existed set only HTTPAlias, and must
// still get the pair it always got.
func TestEmptyTransportFieldsKeepTheNetHTTPShape(t *testing.T) {
	e := NewEmitter()
	e.Symbols.HTTPImport = "net/http"
	e.Symbols.HTTPAlias = "nethttp"
	e.Symbols.RequestType = ""
	e.Symbols.HandlerParams = ""

	symbols := e.symbols()
	if got, want := symbols.RequestType, "*nethttp.Request"; got != want {
		t.Errorf("RequestType = %q, want %q", got, want)
	}
	if got, want := symbols.HandlerParams, "w nethttp.ResponseWriter, r *nethttp.Request"; got != want {
		t.Errorf("HandlerParams = %q, want %q", got, want)
	}
	if got, want := symbols.TransportArgs(), "w, r"; got != want {
		t.Errorf("TransportArgs = %q, want %q", got, want)
	}
}

// writeLogic puts one page.go in a temp dir and returns its path.
func writeLogic(t *testing.T, source string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "page.go")
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

const fastHandlerPage = `package p

import "github.com/shibukawa/tinygodriver/fasthttp"

func Load(ctx *fasthttp.RequestCtx) {}
`

const netHandlerPage = `package p

import "net/http"

func Load(w http.ResponseWriter, r *http.Request) {}
`

// TestHandlerShapeRecognizesTheConfiguredTransport is why the shape is
// configuration: a recognizer keyed on net/http alone reads a correct fasthttp
// handler as a malformed typed page.
func TestHandlerShapeRecognizesTheConfiguredTransport(t *testing.T) {
	fastPath := writeLogic(t, fastHandlerPage)

	fn, err := InspectLogicWith(fastPath, FastHTTPHandlerShape(""))
	if err != nil {
		t.Fatalf("InspectLogicWith: %v", err)
	}
	if fn.Rung != RungHandlerPage {
		t.Errorf("rung = %v, want %v", fn.Rung, RungHandlerPage)
	}

	// The same file under the other transport's shape is not a handler, and the
	// error names the signature it could have been.
	if _, err := InspectLogic(fastPath); err == nil {
		t.Error("a fasthttp handler was accepted as a net/http typed page")
	} else if !strings.Contains(err.Error(), "func(http.ResponseWriter, *http.Request)") {
		t.Errorf("the error does not name the expected shape: %v", err)
	}
}

func TestHandlerShapeRejectsTheOtherTransport(t *testing.T) {
	netPath := writeLogic(t, netHandlerPage)

	_, err := InspectLogicWith(netPath, FastHTTPHandlerShape(""))
	if err == nil {
		t.Fatal("a net/http handler was accepted under the fasthttp shape")
	}
	if !strings.Contains(err.Error(), "func(*fasthttp.RequestCtx)") {
		t.Errorf("the error does not name the fasthttp shape: %v", err)
	}
}

// TestZeroHandlerShapeIsNetHTTP keeps every existing caller on the shape it had
// before the field existed.
func TestZeroHandlerShapeIsNetHTTP(t *testing.T) {
	fn, err := InspectLogicWith(writeLogic(t, netHandlerPage), HandlerShape{})
	if err != nil {
		t.Fatalf("InspectLogicWith: %v", err)
	}
	if fn.Rung != RungHandlerPage {
		t.Errorf("rung = %v, want %v", fn.Rung, RungHandlerPage)
	}
	if got := NewEmitter().handlerShape().Import; got != "net/http" {
		t.Errorf("default emitter handler shape imports %q", got)
	}
}

// TestRoutePatternsDefaultToNetHTTPSpelling keeps the pattern seam invisible to
// every caller that has not asked for it.
func TestRoutePatternsDefaultToNetHTTPSpelling(t *testing.T) {
	symbols := DefaultSymbols()
	for _, tc := range []struct{ path, want string }{
		{"/", "GET /{$}"},
		{"/about", "GET /about"},
		{"/users/{id}", "GET /users/{id}"},
		{"/files/{rest...}", "GET /files/{rest...}"},
	} {
		if got := symbols.RoutePattern(Route{Path: tc.path}); got != tc.want {
			t.Errorf("RoutePattern(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
	// The fasthttp preset targets a router reading Go 1.22 patterns, so it
	// changes nothing here either.
	if got := FastHTTPSymbols("").RoutePattern(Route{Path: "/files/{rest...}"}); got != "GET /files/{rest...}" {
		t.Errorf("the fasthttp preset rewrote a pattern its target reads as written: %q", got)
	}
}

// TestRoutePatternsFollowTheRouter covers the two spellings a router that does
// not read Go 1.22 patterns needs. Both are silent failures rather than loud
// ones if left alone: such a router reads "{rest...}" as a parameter named
// "rest..." and "{$}" as one named "$".
func TestRoutePatternsFollowTheRouter(t *testing.T) {
	symbols := FastHTTPSymbols("")
	symbols.CatchAllSuffix = ":*"
	symbols.RootPattern = "/"

	for _, tc := range []struct{ path, want string }{
		{"/", "GET /"},
		{"/users/{id}", "GET /users/{id}"},
		{"/files/{rest...}", "GET /files/{rest:*}"},
		{"/a/{x}/b/{rest...}", "GET /a/{x}/b/{rest:*}"},
	} {
		if got := symbols.RoutePattern(Route{Path: tc.path}); got != tc.want {
			t.Errorf("RoutePattern(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

// TestRegistryWritesTheRoutersSpelling is the same rule where it matters: in
// the registration the router actually reads, with the declared address kept
// beside it for a sitemap.
func TestRegistryWritesTheRoutersSpelling(t *testing.T) {
	home, homeAnalysis := templateOnly("/", "", "pages", "example.com/m/pages", nil, nil)
	files, filesAnalysis := templateOnly("/files/{rest...}", "files/rest__", "rest__", "example.com/m/pages/files/rest__",
		[]Segment{catchAll("rest")}, []Value{{Name: "rest", Type: "string"}})

	e := NewFastHTTPEmitter("")
	e.Symbols.CatchAllSuffix = ":*"
	e.Symbols.RootPattern = "/"
	source := registry(t, e, &Tree{Routes: []Route{home, files}},
		[]Analysis{homeAnalysis, filesAnalysis}, nil)

	mustContain(t, source,
		`mux.HandleFunc("GET /",`,
		`mux.HandleFunc("GET /files/{rest:*}",`,
		// The table keeps the declared address, because a router spelling is not
		// a URL and a sitemap needs the URL.
		`{Pattern: "GET /files/{rest:*}", Path: "/files/{rest...}"`,
	)
}
