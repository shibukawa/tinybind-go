package routetree

import (
	"os"
	"path/filepath"
	"testing"
)

func logicFile(t *testing.T, source string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), DefaultLogicFile)
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func inspect(t *testing.T, source string) *PageFunc {
	t.Helper()
	fn, err := InspectLogic(logicFile(t, source))
	if err != nil {
		t.Fatalf("InspectLogic: %v", err)
	}
	return fn
}

func TestInspectLogicNoFileIsTemplateOnly(t *testing.T) {
	fn, err := InspectLogic("")
	if err != nil {
		t.Fatalf("InspectLogic: %v", err)
	}
	if fn.Rung != RungTemplateOnly {
		t.Errorf("Rung = %v, want %v", fn.Rung, RungTemplateOnly)
	}
}

func TestInspectLogicFileWithoutPageIsTemplateOnly(t *testing.T) {
	fn := inspect(t, `package id_

func helper() string { return "" }
`)
	if fn.Rung != RungTemplateOnly {
		t.Errorf("Rung = %v, want %v", fn.Rung, RungTemplateOnly)
	}
}

func TestInspectLogicHandlerPage(t *testing.T) {
	fn := inspect(t, `package id_

import "net/http"

func Load(w http.ResponseWriter, r *http.Request) {}
`)
	if fn.Rung != RungHandlerPage {
		t.Errorf("Rung = %v, want %v", fn.Rung, RungHandlerPage)
	}
}

func TestInspectLogicHandlerPageWithAliasedImport(t *testing.T) {
	fn := inspect(t, `package id_

import nethttp "net/http"

func Load(w nethttp.ResponseWriter, r *nethttp.Request) {}
`)
	if fn.Rung != RungHandlerPage {
		t.Errorf("Rung = %v, want %v", fn.Rung, RungHandlerPage)
	}
}

func TestInspectLogicRejectsHandlerShapeWithoutTheImport(t *testing.T) {
	// Without the net/http import these are two ordinary named types, so the
	// declaration is read as a typed Page and fails on its missing error.
	_, err := InspectLogic(logicFile(t, `package id_

func Load(w http.ResponseWriter, r *http.Request) {}
`))
	if err == nil {
		t.Fatal("accepted, want rejection")
	}
}

func TestInspectLogicReportsParseErrors(t *testing.T) {
	if _, err := InspectLogic(logicFile(t, "package id_\n\nfunc Load( {")); err == nil {
		t.Fatal("unparsable file accepted, want error")
	}
}

// route builds a route with the given dynamic parameter names.
func routeWithParams(path string, names ...string) Route {
	route := Route{Path: path}
	for _, name := range names {
		route.Params = append(route.Params, Segment{Dir: name + "_", Name: name, Kind: DynamicSegment})
	}
	return route
}
