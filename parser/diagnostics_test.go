package parser_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shibukawa/tinybind-go/parser"
)

func TestCheckPackage_DynamicPatternDiagnostic(t *testing.T) {
	dir := t.TempDir()
	writeStrictTestModule(t, dir)
	src := `package app

import (
	"net/http"

	"github.com/shibukawa/tinybind-go/httpbind"
)

type Req struct{ Name string }
type Resp struct{ ID string }

func good(w http.ResponseWriter, r *http.Request) {
	_, _ = httpbind.Bind[Req](r)
	_ = httpbind.Write[Resp](w, r, Resp{ID: "1"})
}

func bad(w http.ResponseWriter, r *http.Request) {
	_, _ = httpbind.Bind[Req](r)
	_ = httpbind.Write[Resp](w, r, Resp{})
}

func register(mux *http.ServeMux) {
	path := "/dynamic"
	mux.HandleFunc("GET "+path, bad) // dynamic pattern → diagnostic
	mux.HandleFunc("POST /ok", good)
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyModule(t, dir)

	res, err := parser.ParsePackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Routes) != 1 || res.Routes[0].Path != "/ok" {
		t.Fatalf("want only /ok route, got %+v", res.Routes)
	}
	if len(res.Diagnostics) == 0 {
		t.Fatal("expected dynamic_pattern diagnostic")
	}
	found := false
	for _, d := range res.Diagnostics {
		if d.Reason == parser.ReasonDynamicPattern {
			found = true
			if d.Line <= 0 || d.Message == "" {
				t.Fatalf("incomplete diagnostic: %+v", d)
			}
			if !d.OmitsOpenAPI {
				t.Fatalf("dynamic pattern should omit OpenAPI: %+v", d)
			}
		}
	}
	if !found {
		t.Fatalf("diagnostics: %+v", res.Diagnostics)
	}

	diags, err := parser.CheckPackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(diags) == 0 {
		t.Fatal("CheckPackage should return diagnostics")
	}
}

func TestCheckPackage_CleanPackageNoDiagnostics(t *testing.T) {
	dir := filepath.Join("..", "testdata", "basic_handlefunc")
	diags, err := parser.CheckPackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}
}

func TestParsePackage_WriteStatusSuccessStatuses(t *testing.T) {
	dir := t.TempDir()
	writeStrictTestModule(t, dir)
	src := `package app

import (
	"net/http"

	"github.com/shibukawa/tinybind-go/httpbind"
)

type CreateReq struct{ Name string }
type CreateResp struct{ ID string }

func create(w http.ResponseWriter, r *http.Request) {
	in, err := httpbind.Bind[CreateReq](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	_ = in
	_ = httpbind.WriteStatus[CreateResp](w, r, http.StatusCreated, CreateResp{ID: "1"})
}

func register(mux *http.ServeMux) {
	mux.HandleFunc("POST /items", create)
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyModule(t, dir)
	res, err := parser.ParsePackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Routes) != 1 {
		t.Fatalf("routes: %+v", res.Routes)
	}
	st := res.Routes[0].SuccessStatuses
	if len(st) != 1 || st[0] != 201 {
		t.Fatalf("success_statuses: %v", st)
	}
	if res.Routes[0].Response != "CreateResp" {
		t.Fatalf("response: %s", res.Routes[0].Response)
	}
}
