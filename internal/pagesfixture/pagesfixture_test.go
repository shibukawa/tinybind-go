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
	"strings"
	"testing"

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

func TestDiscoveryDefaultsToPages(t *testing.T) {
	if routetree.DefaultRootDir != "pages" {
		t.Errorf("DefaultRootDir = %q, want pages", routetree.DefaultRootDir)
	}
}
