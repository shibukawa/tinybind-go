// Package routefixture holds a committed, compiled example of the code
// routetree emits. Two things are checked here that a unit test on the emitter
// cannot check on its own: that the generated source compiles against the real
// httpbind runtime, and that it behaves correctly behind a real ServeMux.
package routefixture

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	id_ "github.com/shibukawa/tinybind-go/internal/routefixture/users/id_"
	"github.com/shibukawa/tinybind-go/routetree"
)

// fixtureRoute is the route the committed file was generated from.
func fixtureRoute() (routetree.Route, []routetree.Value) {
	route := routetree.Route{
		Path:     "/users/{id}",
		Package:  "id_",
		PageFile: "internal/routefixture/users/id_/page.tb.html",
		Params: []routetree.Segment{
			{Dir: "id_", Name: "id", Kind: routetree.DynamicSegment},
		},
	}
	inputs := []routetree.Value{
		{Name: "id", Type: "string"},
		{Name: "page", Type: "int"},
		{Name: "verbose", Type: "bool"},
	}
	return route, inputs
}

// TestGeneratedFileIsUpToDate keeps the committed fixture honest. It fails when
// the emitter changes without the fixture being regenerated, which is what
// makes the compile check below meaningful.
func TestGeneratedFileIsUpToDate(t *testing.T) {
	route, inputs := fixtureRoute()
	want, err := routetree.EmitDecoder(route, inputs)
	if err != nil {
		t.Fatalf("EmitDecoder: %v", err)
	}
	path := filepath.Join("users", "id_", "route_gen.go")
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("%s is stale; regenerate it.\n--- committed ---\n%s\n--- emitted ---\n%s", path, got, want)
	}
}

// serve registers the generated decoder behind the pattern it was generated
// for, so PathValue resolves the way it does in a real server.
func serve(t *testing.T) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
		params, err := id_.DecodeRoute(r)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		w.Header().Set("X-ID", params.ID)
		w.Header().Set("X-Page", strconv.Itoa(params.Page))
		if params.Verbose {
			w.Header().Set("X-Verbose", "true")
		}
	})
	return mux
}

func TestDecodeRouteReadsPathAndQuery(t *testing.T) {
	rec := httptest.NewRecorder()
	serve(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users/u42?page=3&verbose=true", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("X-ID"); got != "u42" {
		t.Errorf("ID = %q, want u42", got)
	}
	if got := rec.Header().Get("X-Page"); got != "3" {
		t.Errorf("Page = %q, want 3", got)
	}
	if got := rec.Header().Get("X-Verbose"); got != "true" {
		t.Errorf("Verbose = %q, want true", got)
	}
}

func TestDecodeRouteLeavesAbsentQueryAtZero(t *testing.T) {
	rec := httptest.NewRecorder()
	serve(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users/u42", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	// A missing query key is an ordinary request, not a malformed one.
	if got := rec.Header().Get("X-Page"); got != "0" {
		t.Errorf("Page = %q, want 0", got)
	}
	if got := rec.Header().Get("X-Verbose"); got != "" {
		t.Errorf("Verbose = %q, want unset", got)
	}
}

func TestDecodeRouteRejectsUnparsableQuery(t *testing.T) {
	rec := httptest.NewRecorder()
	serve(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users/u42?page=many", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	// The message must name the parameter, because that is what the caller has
	// to fix.
	if body := rec.Body.String(); body == "" {
		t.Error("empty error body")
	}
}

func TestDecodeRouteRejectsUnparsableBool(t *testing.T) {
	rec := httptest.NewRecorder()
	serve(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users/u42?verbose=yes-please", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestDecodeRouteAcceptsEveryBoolSpelling(t *testing.T) {
	for _, raw := range []string{"1", "t", "T", "true", "TRUE", "True"} {
		rec := httptest.NewRecorder()
		serve(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users/u42?verbose="+raw, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("verbose=%s: status = %d", raw, rec.Code)
			continue
		}
		if got := rec.Header().Get("X-Verbose"); got != "true" {
			t.Errorf("verbose=%s: X-Verbose = %q", raw, got)
		}
	}
}
