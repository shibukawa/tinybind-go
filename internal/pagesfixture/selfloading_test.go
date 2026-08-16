package pagesfixture

import (
	"net/http"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/htmlbind/delta"
	records "github.com/shibukawa/tinybind-go/internal/pagesfixture/pages/records/id_"
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

// Loading its own data must not cost the page its update boundary.
//
// Reported by the framework 2026-08-14: hoisting moves a binding in front of
// the block's markup, so the page presented a value binding where its root
// element used to be and the emitter read that as "no single root". Nothing
// failed — the page rendered, and only the delta path was quietly degraded.
// This is the shape the retired typed rung leaves behind, so it is the shape
// worth holding: one root element, and a binding inside it that reaches past
// it.
//
// It runs the generated code rather than reading the generated file, because a
// golden regenerated with the defect in place is what let this ship.
func TestASelfLoadingPageIsStillAnUpdateBoundary(t *testing.T) {
	var out strings.Builder
	manifest, err := delta.CollectChain(&out, []byte("k"), nil, records.Page(records.PageParams{Id: "seven"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Instances) != 1 {
		t.Fatalf("the self-loading page contributed no boundary: %+v\n%s", manifest.Instances, out.String())
	}
	instance := manifest.Instances[0]
	if instance.ComponentID != "templates.page.Page" {
		t.Errorf("ComponentID = %q", instance.ComponentID)
	}
	if instance.FrameValidator == "" {
		t.Error("the boundary carries no frame validator, so a navigation cannot compare it")
	}
	// The attribute lands on the root element the binding encloses, which is
	// what makes the region addressable in the browser.
	if want := `<section data-tb-id="` + instance.ID + `">`; !strings.Contains(out.String(), want) {
		t.Errorf("body = %q, want it to contain %q", out.String(), want)
	}
}
