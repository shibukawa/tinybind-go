package pagesfixture

import (
	"net/http"
	"strings"
	"testing"
)

// A page that loads its own data needs no typed entry point. The component
// takes the path parameter, binds the loader's result, and the route package
// declares no func Load at all — which is what makes the typed rung's remaining
// job small enough to retire.
func TestARouteLoadsItsOwnDataWithNoTypedEntryPoint(t *testing.T) {
	rec := get(t, serveMux(), "/records/seven")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	for _, want := range []string{"record seven", "summary of seven"} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("body = %q, want it to contain %q", rec.Body, want)
		}
	}
}

// The loader's error chooses the response from inside the component. It works
// because decision:value-binding-hoisting runs a leaf's leading bindings during
// assembly, so the status is still free when the loader fails.
func TestASelfLoadingRouteChoosesItsOwnResponse(t *testing.T) {
	for name, expect := range map[string]struct {
		target   string
		status   int
		location string
	}{
		"a not found": {"/records/missing", http.StatusNotFound, ""},
		"a redirect":  {"/records/moved", http.StatusSeeOther, "/records/here"},
	} {
		t.Run(name, func(t *testing.T) {
			rec := get(t, serveMux(), expect.target)
			if rec.Code != expect.status {
				t.Fatalf("status = %d, want %d; body = %s", rec.Code, expect.status, rec.Body)
			}
			if expect.location != "" && rec.Header().Get("Location") != expect.location {
				t.Fatalf("Location = %q, want %q", rec.Header().Get("Location"), expect.location)
			}
			// Nothing of the page may have been written, or the status could not
			// have been chosen at all.
			if strings.Contains(rec.Body.String(), "<h1>record") {
				t.Fatalf("the page rendered before its loader failed: %s", rec.Body)
			}
		})
	}
}
