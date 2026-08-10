// Package fasthttppagesfixture is a whole route tree generated for the second
// transport.
//
// It is the fasthttp half of internal/pagesfixture, and it proves the thing a
// unit test on the emitter cannot: that a page tree emitted for fasthttp
// compiles against the real fasthttpbind runtime and serves through a router of
// the emitted interface's shape. Until this existed, a fasthttp build of a
// project with a page tree had no routes and no decoders at all.
package fasthttppagesfixture

import (
	"os"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/internal/fasthttppagesfixture/pages"
	id_ "github.com/shibukawa/tinybind-go/internal/fasthttppagesfixture/pages/users/id_"
	"github.com/shibukawa/tinybind-go/routetree"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

const importBase = "github.com/shibukawa/tinybind-go/internal/fasthttppagesfixture/pages"

func options() routetree.GenerateOptions {
	return routetree.GenerateOptions{
		Config:      routetree.Config{Root: "pages", ImportBase: importBase},
		RootPackage: "pages",
		Emitter:     routetree.NewFastHTTPEmitter(""),
	}
}

// TestGeneratedFilesAreUpToDate keeps the committed output honest, so the serve
// checks below are testing the current emitter.
//
// Set REGEN=1 to rewrite them after an intentional change.
func TestGeneratedFilesAreUpToDate(t *testing.T) {
	files, err := routetree.Generate(options())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("Generate produced nothing")
	}
	if os.Getenv("REGEN") != "" {
		if err := routetree.Write(files); err != nil {
			t.Fatal(err)
		}
		t.Log("regenerated")
		return
	}
	for _, file := range files {
		committed, err := os.ReadFile(file.Path)
		if err != nil {
			t.Errorf("%s: %v", file.Path, err)
			continue
		}
		if string(committed) != string(file.Source) {
			t.Errorf("%s is stale; rerun with REGEN=1.\n--- committed ---\n%s\n--- emitted ---\n%s",
				file.Path, committed, file.Source)
		}
	}
}

// TestNoGeneratedFileNamesNetHTTP is the negative half of the port: a page tree
// emitted for this transport must not reach for the other one, because the two
// runtimes are chosen at generation time and a file naming both would not
// compile in either build.
func TestNoGeneratedFileNamesNetHTTP(t *testing.T) {
	files, err := routetree.Generate(options())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for _, file := range files {
		if strings.Contains(string(file.Source), `"net/http"`) {
			t.Errorf("%s imports net/http:\n%s", file.Path, file.Source)
		}
	}
}

// router is the shape the generated registry asks for. The emitted parameter is
// a one-method interface rather than a concrete type, because fasthttp ships no
// router and naming one would decide which third-party package an application
// depends on.
//
// This one matches literally, without pattern translation, which is all the
// fixture needs; a real router reads the Go 1.22 patterns the emitter writes.
type router struct {
	handlers map[string]func(*fasthttp.RequestCtx)
}

func newRouter() *router {
	return &router{handlers: map[string]func(*fasthttp.RequestCtx){}}
}

func (r *router) HandleFunc(pattern string, handler func(*fasthttp.RequestCtx)) {
	r.handlers[pattern] = handler
}

// serve runs one request through the registered handler, with the path values a
// real router would have stored. fasthttp has no routing of its own, so a path
// value reaches the decoder as a user value rather than off the request.
func (r *router) serve(t *testing.T, pattern, uri string, pathValues map[string]string) *fasthttp.RequestCtx {
	t.Helper()
	handler, ok := r.handlers[pattern]
	if !ok {
		t.Fatalf("no handler registered for %q", pattern)
	}
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.SetRequestURI(uri)
	for key, value := range pathValues {
		ctx.SetUserValue(key, value)
	}
	handler(ctx)
	return ctx
}

func registered(t *testing.T) *router {
	t.Helper()
	mux := newRouter()
	pages.Register(mux)
	return mux
}

func TestRegisterInstallsEveryRoute(t *testing.T) {
	mux := registered(t)
	for _, pattern := range []string{"GET /{$}", "GET /raw", "GET /users/{id}", "GET /files/{rest...}"} {
		if _, ok := mux.handlers[pattern]; !ok {
			t.Errorf("no handler for %q; registered: %v", pattern, mux.handlers)
		}
	}
}

func TestGeneratedHandlerRendersThroughTheLayoutChain(t *testing.T) {
	ctx := registered(t).serve(t, "GET /users/{id}", "/users/u42?page=3", map[string]string{"id": "u42"})

	body := string(ctx.Response.Body())
	for _, want := range []string{`id="shell"`, "U42", "page 3"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q:\n%s", want, body)
		}
	}
}

// TestGeneratedHandlerPassesTheRequestAsTheContext is the transport difference
// the rewrite table names: net/http reads a context off the request with a
// call, and this transport's request value is one already.
func TestGeneratedHandlerPassesTheRequestAsTheContext(t *testing.T) {
	mux := registered(t)
	handler := mux.handlers["GET /users/{id}"]

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.SetRequestURI("/users/u42")
	ctx.SetUserValue("id", "u42")
	id_.WithReader(ctx, "ada")
	handler(ctx)

	if body := string(ctx.Response.Body()); !strings.Contains(body, "for ada") {
		t.Errorf("the request-scoped value did not reach the typed entry point:\n%s", body)
	}
}

func TestAbsentOptionalQueryStaysNil(t *testing.T) {
	ctx := registered(t).serve(t, "GET /users/{id}", "/users/u42", map[string]string{"id": "u42"})

	if body := string(ctx.Response.Body()); !strings.Contains(body, "every page") {
		t.Errorf("absent optional query did not bind nil:\n%s", body)
	}
}

func TestUnparsableQueryIsRejected(t *testing.T) {
	ctx := registered(t).serve(t, "GET /users/{id}", "/users/u42?page=many", map[string]string{"id": "u42"})

	if got := ctx.Response.StatusCode(); got != fasthttp.StatusBadRequest {
		t.Fatalf("status = %d, want 400", got)
	}
	if body := string(ctx.Response.Body()); !strings.Contains(body, "page") {
		t.Errorf("the error does not name the parameter:\n%s", body)
	}
}

// TestCatchAllBindsTheRemainder covers the one segment shape whose spelling can
// differ between routers. The default target reads Go 1.22 patterns, so the
// pattern registers as written; what this checks is the other half, that the
// decoder reads the value under the segment's name rather than under whatever
// the pattern happens to spell.
func TestCatchAllBindsTheRemainder(t *testing.T) {
	ctx := registered(t).serve(t, "GET /files/{rest...}", "/files/docs/readme.md",
		map[string]string{"rest": "docs/readme.md"})

	if body := string(ctx.Response.Body()); !strings.Contains(body, "path: docs/readme.md") {
		t.Errorf("the catch-all remainder did not reach the page:\n%s", body)
	}
}

// TestRouteTableSeparatesPatternFromPath states which column a sitemap reads. A
// router's own catch-all spelling is not a URL, so Path stays as the filesystem
// declared it whatever the router wants.
func TestRouteTableSeparatesPatternFromPath(t *testing.T) {
	for _, route := range pages.Routes {
		if route.Dir != "files/rest__" {
			continue
		}
		if route.Path != "/files/{rest...}" {
			t.Errorf("Path = %q, want the declared address", route.Path)
		}
		if len(route.Params) != 1 || route.Params[0] != "rest" {
			t.Errorf("Params = %v, want [rest]", route.Params)
		}
		return
	}
	t.Fatalf("the catch-all route is missing from the table: %v", pages.Routes)
}

// TestRawRouteIsRegisteredUnwrapped covers the rung the recognizer had to be
// taught: a Load of this transport's handler shape owns its whole response, so
// the registry contributes registration and nothing else.
func TestRawRouteIsRegisteredUnwrapped(t *testing.T) {
	ctx := registered(t).serve(t, "GET /raw", "/raw", nil)

	if got := ctx.Response.StatusCode(); got != fasthttp.StatusTeapot {
		t.Fatalf("status = %d, want %d", got, fasthttp.StatusTeapot)
	}
	if body := string(ctx.Response.Body()); body != "raw" {
		t.Errorf("body = %q, want raw", body)
	}
}

// TestServerActionIsDiscovered covers the other user of the handler shape:
// an exported handler in a route package is an endpoint whether or not a
// template names it.
func TestServerActionIsDiscovered(t *testing.T) {
	var pattern string
	for _, action := range pages.Actions {
		if action.Handler == "Rename" {
			pattern = action.Pattern
		}
	}
	if pattern == "" {
		t.Fatalf("Rename was not discovered; actions: %v", pages.Actions)
	}

	mux := registered(t)
	handler, ok := mux.handlers[pattern]
	if !ok {
		t.Fatalf("no handler registered for %q", pattern)
	}
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.SetRequestURI(strings.Split(pattern, " ")[1] + "?name=ada")
	handler(ctx)

	if body := string(ctx.Response.Body()); body != "renamed to ADA" {
		t.Errorf("body = %q", body)
	}
}
