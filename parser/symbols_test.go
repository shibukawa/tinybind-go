package parser_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/shibukawa/tinybind-go/parser"
)

func writeStrictTestModule(t *testing.T, dir string) {
	t.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	mod := "module stricttest\n\ngo 1.25\n\nrequire github.com/shibukawa/tinybind-go v0.0.0\n\nreplace github.com/shibukawa/tinybind-go => " + filepath.ToSlash(root) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o644); err != nil {
		t.Fatal(err)
	}
}

// tidyModule runs go mod tidy after sources exist in dir.
// skipWithoutToolchain skips a test that shells out to the Go toolchain. Those
// tests tidy or compile a temp module against this one, which costs seconds
// apiece and dominates this package's runtime. Short mode is the fast loop; a
// full run skips nothing.
func skipWithoutToolchain(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("short mode: this test runs the Go toolchain against a temp module")
	}
}

func tidyModule(t *testing.T, dir string) {
	t.Helper()
	skipWithoutToolchain(t)
	t.Helper()
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy: %v\n%s", err, out)
	}
}

func TestParsePackage_RejectsForeignHandleFunc(t *testing.T) {
	dir := t.TempDir()
	writeStrictTestModule(t, dir)
	src := `package app

import (
	"net/http"

	"github.com/shibukawa/tinybind-go"
)

type FakeMux struct{}

func (FakeMux) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {}

type Req struct{ X string }
type Resp struct{ Y string }

func h(w http.ResponseWriter, r *http.Request) {
	_, _ = httpbind.Bind[Req](r)
	_ = httpbind.Write[Resp](w, r, Resp{})
}

func register() {
	var fake FakeMux
	fake.HandleFunc("POST /fake", h) // must NOT discover

	mux := http.NewServeMux()
	mux.HandleFunc("POST /real", h) // must discover
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyModule(t, dir)
	got, err := parser.ParsePackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Routes) != 1 {
		t.Fatalf("want 1 route (real mux only), got %d: %+v", len(got.Routes), got.Routes)
	}
	if got.Routes[0].Path != "/real" {
		t.Fatalf("path: %s", got.Routes[0].Path)
	}
}

func TestParsePackage_AliasImportRecognized(t *testing.T) {
	dir := t.TempDir()
	writeStrictTestModule(t, dir)
	src := `package app

import (
	"net/http"

	hb "github.com/shibukawa/tinybind-go"
)

type Req struct{ Name string }
type Resp struct{ ID string }

func h(w http.ResponseWriter, r *http.Request) {
	_, err := hb.Bind[Req](r)
	if err != nil {
		_ = hb.BadRequest(hb.Problem{Code: "x", Message: "y"})
		hb.WriteError(w, r, err)
		return
	}
	_ = hb.Write[Resp](w, r, Resp{ID: "1"})
}

func register(mux *http.ServeMux) {
	mux.HandleFunc("POST /alias", h)
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyModule(t, dir)
	got, err := parser.ParsePackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Routes) != 1 {
		t.Fatalf("routes: %+v", got.Routes)
	}
	rt := got.Routes[0]
	if rt.Request != "Req" || rt.Response != "Resp" {
		t.Fatalf("models: req=%q resp=%q", rt.Request, rt.Response)
	}
	found := false
	for _, e := range rt.Errors {
		if e == "BadRequest" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected BadRequest via alias, got %v", rt.Errors)
	}
}

func TestParsePackage_RejectsForeignBindAndError(t *testing.T) {
	dir := t.TempDir()
	writeStrictTestModule(t, dir)
	// otherpkg in same module path-style: define local funcs with same names.
	src := `package app

import (
	"net/http"

	"github.com/shibukawa/tinybind-go"
)

type Req struct{ Name string }
type Resp struct{ ID string }

func Bind[T any](r *http.Request) (T, error) {
	var zero T
	return zero, nil
}

func BadRequest(msg string) error { return nil }

func Write[T any](w http.ResponseWriter, r *http.Request, v T) error { return nil }

func h(w http.ResponseWriter, r *http.Request) {
	// Local same-named generics/functions — must not count as httpbind.
	_, _ = Bind[Req](r)
	_ = BadRequest("nope")
	_ = Write[Resp](w, r, Resp{})

	// Real httpbind call for request only
	in, err := httpbind.Bind[Req](r)
	_ = in
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	_ = httpbind.Write[Resp](w, r, Resp{ID: "1"})
}

func register(mux *http.ServeMux) {
	mux.HandleFunc("GET /mixed", h)
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyModule(t, dir)
	got, err := parser.ParsePackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Routes) != 1 {
		t.Fatalf("routes: %+v", got.Routes)
	}
	rt := got.Routes[0]
	if rt.Request != "Req" || rt.Response != "Resp" {
		t.Fatalf("models: %+v", rt)
	}
	if len(rt.Errors) != 0 {
		t.Fatalf("local BadRequest must not be discovered, got %v", rt.Errors)
	}
}

func TestParsePackage_HTTPPackageHandleFunc(t *testing.T) {
	dir := t.TempDir()
	writeStrictTestModule(t, dir)
	src := `package app

import (
	"net/http"

	"github.com/shibukawa/tinybind-go"
)

type Req struct{}
type Resp struct{}

func h(w http.ResponseWriter, r *http.Request) {
	_, _ = httpbind.Bind[Req](r)
	_ = httpbind.Write[Resp](w, r, Resp{})
}

func register() {
	http.HandleFunc("GET /health", h)
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyModule(t, dir)
	got, err := parser.ParsePackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Routes) != 1 || got.Routes[0].Path != "/health" {
		t.Fatalf("routes: %+v", got.Routes)
	}
}

func TestParsePackage_CustomSymbols(t *testing.T) {
	dir := t.TempDir()
	writeStrictTestModule(t, dir)
	src := `package app
import "net/http"
type Router struct{}
func (Router) Route(pattern string, h func(http.ResponseWriter,*http.Request)){}
type Req struct{ Name string }
type Resp struct{ ID string }
func Bind[T any](*http.Request)(T,error){var z T;return z,nil}
func Write[T any](http.ResponseWriter,*http.Request,T)error{return nil}
func h(w http.ResponseWriter,r *http.Request){_,_=Bind[Req](r);_=Write[Resp](w,r,Resp{})}
func register(router Router){router.Route("POST /custom",h)}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyModule(t, dir)
	got, err := parser.ParsePackageWithConfig(dir, parser.Config{
		Calls: []parser.CallPattern{
			{
				Target: parser.RouteSymbol{
					PackagePath: "stricttest", Name: "Route",
					ReceiverPackagePath: "stricttest", ReceiverType: "Router",
				},
				Operation: parser.CallRouteRegister, PatternArgument: 0, HandlerArgument: 1,
			},
			{
				Target:    parser.RouteSymbol{PackagePath: "stricttest", Name: "Bind"},
				Operation: parser.CallRequestBind,
			},
			{
				Target:    parser.RouteSymbol{PackagePath: "stricttest", Name: "Write"},
				Operation: parser.CallResponseWrite,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Routes) != 1 || got.Routes[0].Path != "/custom" {
		t.Fatalf("routes: %+v", got.Routes)
	}
	if got.Routes[0].Request != "Req" || got.Routes[0].Response != "Resp" {
		t.Fatalf("models: %+v", got.Routes[0])
	}
}
