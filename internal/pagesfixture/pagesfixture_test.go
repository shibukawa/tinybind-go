// Package pagesfixture is a whole route tree generated end to end.
//
// It proves what no unit test can: that discovery, template compilation,
// decoder emission, and registry emission produce Go that compiles together and
// serves real HTML through a real ServeMux.
package pagesfixture

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
	"github.com/shibukawa/tinybind-go/internal/pagesfixture/pages"
	"github.com/shibukawa/tinybind-go/routetree"
)

const importBase = "github.com/shibukawa/tinybind-go/internal/pagesfixture/pages"

func options() routetree.GenerateOptions {
	return routetree.GenerateOptions{
		Config:      routetree.Config{Root: "pages", ImportBase: importBase},
		RootPackage: "pages",
	}
}

// TestGeneratedFilesAreUpToDate keeps the committed output honest, so the
// compile and serve checks below are testing the current emitter.
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

// TestBindersAreGeneratedForEveryRoutePackage covers the second half of a
// generation run: a page or a server action may call httpbind.Bind, and the
// binder that dispatches it is generated per package, so the tree's package list
// is what a caller loops over.
//
// It runs on the committed tree rather than a temporary copy, because analysis
// type-checks each package and that needs the tree's own generated files in
// place. Set REGEN=1 to rewrite the binders.
func TestBindersAreGeneratedForEveryRoutePackage(t *testing.T) {
	tree, err := routetree.Discover(options().Config)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	packages := tree.Packages()
	if len(packages) == 0 {
		t.Fatal("the tree reports no packages")
	}

	generated := 0
	for _, pkg := range packages {
		plan, err := generator.New(generator.DefaultOptions()).Analyze(pkg.Dir)
		if err != nil {
			t.Fatalf("%s: %v", pkg.RelDir, err)
		}
		var used bool
		for _, typePlan := range plan.Types {
			if typePlan.Usage != 0 {
				used = true
			}
		}
		if !used {
			// A package with no bound request needs no binder, which is why the
			// loop generates rather than requiring one per package.
			continue
		}
		generated++
		source, err := generator.Emit(plan)
		if err != nil {
			t.Fatalf("%s: %v", pkg.RelDir, err)
		}
		path := filepath.Join(pkg.Dir, "tinybind_gen.go")
		if os.Getenv("REGEN") != "" {
			if err := os.WriteFile(path, source, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		committed, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		if string(committed) != string(source) {
			t.Errorf("%s is stale; rerun with REGEN=1.\n--- committed ---\n%s\n--- emitted ---\n%s",
				path, committed, source)
		}
	}
	if generated == 0 {
		t.Error("no route package needed a binder, so this fixture proves nothing")
	}
}

func get(t *testing.T, mux *http.ServeMux, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func TestServeMuxServesEveryDiscoveredRoute(t *testing.T) {
	mux := pages.NewServeMux()

	cases := map[string]string{
		"/":            "home",
		"/about":       "about",
		"/users/alice": "user ALICE",
	}
	for target, want := range cases {
		rec := get(t, mux, target)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, body = %s", target, rec.Code, rec.Body)
			continue
		}
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("%s: body = %q, want it to contain %q", target, rec.Body, want)
		}
	}
}

func TestRootLayoutWrapsEveryPage(t *testing.T) {
	mux := pages.NewServeMux()
	for _, target := range []string{"/", "/about", "/users/alice"} {
		body := get(t, mux, target).Body.String()
		if !strings.Contains(body, `<div id="shell">`) {
			t.Errorf("%s is not wrapped by the root layout: %s", target, body)
		}
	}
}

func TestTypedPageResultReachesTheComponent(t *testing.T) {
	// func Page uppercases the id, so seeing it in the markup proves the
	// generated handler ran Go between decoding and rendering.
	body := get(t, pages.NewServeMux(), "/users/bob").Body.String()
	if !strings.Contains(body, "user BOB") {
		t.Errorf("typed page result missing: %s", body)
	}
}

func TestQueryParameterReachesATemplateOnlyPage(t *testing.T) {
	body := get(t, pages.NewServeMux(), "/about?topic=routing").Body.String()
	if !strings.Contains(body, "about routing") {
		t.Errorf("query parameter missing: %s", body)
	}
}

func TestAbsentQueryParameterRendersItsZeroValue(t *testing.T) {
	rec := get(t, pages.NewServeMux(), "/about")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "about") {
		t.Errorf("body = %s", rec.Body)
	}
}

// TestOptionalQueryParameterSeparatesAbsentFromZero covers the one thing a
// non-pointer query parameter cannot express: page=0 is a value the author
// chose, and no page at all is not.
func TestOptionalQueryParameterSeparatesAbsentFromZero(t *testing.T) {
	mux := pages.NewServeMux()
	cases := map[string]string{
		"/about":         "every page",
		"/about?page=0":  "page 0",
		"/about?page=":   "every page",
		"/about?page=12": "page 12",
	}
	for target, want := range cases {
		rec := get(t, mux, target)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, body = %s", target, rec.Code, rec.Body)
			continue
		}
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("%s: body = %q, want it to contain %q", target, rec.Body, want)
		}
	}
}

func TestUnparsableOptionalQueryParameterIsStillRejected(t *testing.T) {
	rec := get(t, pages.NewServeMux(), "/about?page=x")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400; body = %s", rec.Code, rec.Body)
	}
}

func TestUnmatchedPathIsNotFound(t *testing.T) {
	if rec := get(t, pages.NewServeMux(), "/nope"); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestNonGetMethodIsRejected(t *testing.T) {
	// The tree registers GET only, so the stdlib mux answers 405 itself.
	rec := httptest.NewRecorder()
	pages.NewServeMux().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/about", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

func TestRegisterInstallsOntoACallerOwnedMux(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	pages.Register(mux)

	if body := get(t, mux, "/health").Body.String(); body != "ok" {
		t.Errorf("existing route lost: %q", body)
	}
	if rec := get(t, mux, "/"); rec.Code != http.StatusOK {
		t.Errorf("generated route missing: %d", rec.Code)
	}
}

func TestRouteTableDescribesTheTree(t *testing.T) {
	byPath := map[string]routetreeInfo{}
	for _, info := range pages.Routes {
		byPath[info.Path] = routetreeInfo{Pattern: info.Pattern, Dir: info.Dir, Params: info.Params}
	}
	if len(byPath) != 3 {
		t.Fatalf("Routes = %+v, want three entries", pages.Routes)
	}
	users, ok := byPath["/users/{id}"]
	if !ok {
		t.Fatalf("dynamic route missing from the table: %+v", pages.Routes)
	}
	if users.Pattern != "GET /users/{id}" {
		t.Errorf("Pattern = %q", users.Pattern)
	}
	if users.Dir != "users/id_" {
		t.Errorf("Dir = %q, want the on-disk directory", users.Dir)
	}
	if len(users.Params) != 1 || users.Params[0] != "id" {
		t.Errorf("Params = %v, want [id]", users.Params)
	}
	// A static route carries no parameters at all, rather than an empty slice
	// that reads as "unknown".
	if home := byPath["/"]; home.Params != nil {
		t.Errorf("root Params = %v, want nil", home.Params)
	}
}

type routetreeInfo struct {
	Pattern string
	Dir     string
	Params  []string
}

func actionPath(t *testing.T, handler string) string {
	t.Helper()
	for _, info := range pages.Actions {
		if info.Handler == handler {
			return info.Path
		}
	}
	t.Fatalf("no endpoint for %s in %+v", handler, pages.Actions)
	return ""
}

func TestServerActionEndpointReachesTheHandler(t *testing.T) {
	rec := httptest.NewRecorder()
	body := strings.NewReader("name=carol")
	request := httptest.NewRequest(http.MethodPost, actionPath(t, "Rename"), body)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	pages.NewServeMux().ServeHTTP(rec, request)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	// The handler owns the whole response, so what it wrote is what came back.
	if got := rec.Body.String(); got != "renamed to CAROL" {
		t.Errorf("body = %q", got)
	}
}

// TestServerActionBindsATypedRequest proves the binder is what read the form:
// the check tag rejects an empty name before the handler writes anything, which
// r.PostFormValue could not do.
func TestServerActionBindsATypedRequest(t *testing.T) {
	rec := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, actionPath(t, "Rename"), strings.NewReader("name="))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	pages.NewServeMux().ServeHTTP(rec, request)

	if rec.Code != http.StatusUnprocessableEntity && rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want a rejection; body = %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "renamed to") {
		t.Errorf("the handler ran on an invalid request: %s", rec.Body)
	}
}

// TestOpenAPIExcludesPageRoutesAndActions holds the line the binder work could
// have crossed: the generated registry registers every page, so analyzing the
// route root must not turn an HTML page into a documented API operation.
func TestOpenAPIExcludesPageRoutesAndActions(t *testing.T) {
	doc, err := generator.BuildOpenAPI("pages")
	if err != nil {
		t.Fatalf("BuildOpenAPI: %v", err)
	}
	paths, ok := doc["paths"].(map[string]any)
	if !ok {
		t.Fatalf("paths = %T", doc["paths"])
	}
	if len(paths) != 0 {
		t.Errorf("page routes and action endpoints entered the document: %v", paths)
	}
}

func TestServerActionEndpointIsPostOnly(t *testing.T) {
	if rec := get(t, pages.NewServeMux(), actionPath(t, "Rename")); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

func TestTemplateCarriesTheLoweredEndpointURL(t *testing.T) {
	body := get(t, pages.NewServeMux(), "/users/alice").Body.String()
	want := `data-tb-action="` + actionPath(t, "Rename") + `"`
	if !strings.Contains(body, want) {
		t.Errorf("page does not carry %s: %s", want, body)
	}
	// The reserved attribute is replaced, never emitted.
	if strings.Contains(body, "server-action") {
		t.Errorf("reserved attribute reached the output: %s", body)
	}
	// Every other attribute survives unread, which is what lets a framework
	// author client behavior in its own vocabulary.
	if !strings.Contains(body, `data-target="#name"`) {
		t.Errorf("sibling attribute dropped: %s", body)
	}
}

func TestUnexportedHandlerIsReachableAtNoURL(t *testing.T) {
	for _, info := range pages.Actions {
		if info.Handler == "internalOnly" {
			t.Fatalf("an unexported handler was published: %+v", info)
		}
	}
}

func TestActionEndpointIsDeterministic(t *testing.T) {
	// No build salt, so a client that cached the URL keeps working across a
	// deploy. Regenerating must reproduce the committed path exactly.
	if got := actionPath(t, "Rename"); got != "/_action/"+strings.Split(got, "/")[2]+"/Rename" {
		t.Errorf("unexpected endpoint shape: %q", got)
	}
	files, err := routetree.Generate(options())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for _, file := range files {
		if !strings.HasSuffix(file.Path, "routes_gen.go") {
			continue
		}
		if !strings.Contains(string(file.Source), actionPath(t, "Rename")) {
			t.Errorf("regeneration produced a different endpoint than the committed one")
		}
	}
}

func TestDiscoveryDefaultsToPages(t *testing.T) {
	if routetree.DefaultRootDir != "pages" {
		t.Errorf("DefaultRootDir = %q, want pages", routetree.DefaultRootDir)
	}
}
