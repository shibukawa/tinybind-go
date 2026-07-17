package generator_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/shibukawa/httpbind-go/generator"
)

func TestOptionsZeroValueDiscoversNothing(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	writeTestFile(t, filepath.Join(dir, "main.go"), `package sample
import hb "github.com/shibukawa/httpbind-go"
type Note struct{ Text string }
func use() { _, _ = hb.DecodeJSON[Note](nil) }
`)
	tidyTempModule(t, dir)
	plan, err := generator.New(generator.Options{}).Analyze(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Types) != 1 || plan.Types[0].Usage != 0 || len(plan.Discovered) != 0 {
		t.Fatalf("zero Options must discover nothing: %+v", plan)
	}
}

func TestPatternSetReplacesDefaultsAndCanListBothPackages(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	if err := os.Mkdir(filepath.Join(dir, "compat"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dir, "compat", "compat.go"), `package compat
import "io"
func DecodeJSON[T any](io.Reader) (T, error) { var zero T; return zero, nil }
`)
	writeTestFile(t, filepath.Join(dir, "main.go"), `package sample
import (
 hb "github.com/shibukawa/httpbind-go"
 "tempmod/compat"
)
type Standard struct{ ID int }
type Compatible struct{ ID int }
func use() { _, _ = hb.DecodeJSON[Standard](nil); _, _ = compat.DecodeJSON[Compatible](nil) }
`)
	tidyTempModule(t, dir)

	onlyCompat := generator.Options{RuntimePackages: generator.PatternSet[string]{Set: []string{"tempmod/compat"}}}
	plan, err := generator.New(onlyCompat).Analyze(dir)
	if err != nil {
		t.Fatal(err)
	}
	assertTypeUsage(t, plan, "Standard", 0)
	assertTypeUsage(t, plan, "Compatible", generator.UsageDecodeJSON)

	both := generator.Options{RuntimePackages: generator.PatternSet[string]{Set: []string{"github.com/shibukawa/httpbind-go", "tempmod/compat"}}}
	plan, err = generator.New(both).Analyze(dir)
	if err != nil {
		t.Fatal(err)
	}
	assertTypeUsage(t, plan, "Standard", generator.UsageDecodeJSON)
	assertTypeUsage(t, plan, "Compatible", generator.UsageDecodeJSON)

	explicitlyEmpty := generator.DefaultOptions()
	explicitlyEmpty.DecodeJSON.Set = []generator.SymbolPattern{}
	plan, err = generator.New(explicitlyEmpty).Analyze(dir)
	if err != nil {
		t.Fatal(err)
	}
	assertTypeUsage(t, plan, "Standard", 0)
	assertTypeUsage(t, plan, "Compatible", 0)
}

func TestDisabledPatternCannotBeReenabledByGenerateAll(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	writeTestFile(t, filepath.Join(dir, "main.go"), "package sample\ntype Note struct{ Text string }\n")
	tidyTempModule(t, dir)
	opts := generator.DefaultOptions()
	opts.DecodeJSON.Disabled = true
	opts.GenerateAll = true
	plan, err := generator.New(opts).Analyze(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := plan.Types[0].Usage; got&generator.UsageDecodeJSON != 0 {
		t.Fatalf("GenerateAll reenabled disabled DecodeJSON: %v", got)
	}
}

func TestCustomServeMuxAndRuntimePackagesBuildOpenAPI(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	if err := os.Mkdir(filepath.Join(dir, "handler"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dir, "handler", "handler.go"), `package handler
import (
 "io"
 "net/http"
)
type ServeMux struct{}
func (*ServeMux) HandleFunc(string, func(http.ResponseWriter, *http.Request)) {}
func DecodeJSON[T any](io.Reader) (T, error) { var zero T; return zero, nil }
func Write[T any](http.ResponseWriter, *http.Request, T) error { return nil }
`)
	writeTestFile(t, filepath.Join(dir, "main.go"), `package sample
import (
 "net/http"
 petit "tempmod/handler"
)
type Request struct{ Name string `+"`json:\"name\"`"+` }
type Response struct{ ID int `+"`json:\"id\"`"+` }
func route(w http.ResponseWriter, r *http.Request) {
 req, _ := petit.DecodeJSON[Request](r.Body)
 _ = req
 _ = petit.Write(w, r, Response{})
}
func register(mux *petit.ServeMux) { mux.HandleFunc("POST /notes", route) }
`)
	tidyTempModule(t, dir)
	opts := generator.DefaultOptions()
	opts.ServeMuxes.Set = []generator.TypePattern{
		{PackagePath: "net/http", Name: "ServeMux"},
		{PackagePath: "tempmod/handler", Name: "ServeMux"},
	}
	opts.RuntimePackages.Set = []string{"github.com/shibukawa/httpbind-go", "tempmod/handler"}
	g := generator.New(opts)
	plan, err := g.Analyze(dir)
	if err != nil {
		t.Fatal(err)
	}
	assertTypeUsage(t, plan, "Request", generator.UsageDecodeJSON)
	assertTypeUsage(t, plan, "Response", generator.UsageWrite)
	doc, err := g.BuildOpenAPI(dir)
	if err != nil {
		t.Fatal(err)
	}
	paths := doc["paths"].(map[string]any)
	if _, ok := paths["/notes"]; !ok {
		t.Fatalf("custom ServeMux route was not discovered: %#v", paths)
	}
}

func TestRunCannotReenableDisabledOpenAPI(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	writeTestFile(t, filepath.Join(dir, "main.go"), "package sample\ntype Note struct{ Text string }\n")
	tidyTempModule(t, dir)
	opts := generator.DefaultOptions()
	opts.DisableFeatures = []generator.Feature{generator.FeatureOpenAPI}
	var stdout, stderr bytes.Buffer
	code := generator.Run([]string{"-dir", dir, "-out", dir, "-openapi=true"}, &stdout, &stderr, opts)
	if code != 0 {
		t.Fatalf("Run = %d, stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "httpbinder_openapi_gen.go")); !os.IsNotExist(err) {
		t.Fatalf("disabled OpenAPI was generated: %v", err)
	}
	if _, err := generator.New(opts).BuildOpenAPI(dir); !errors.Is(err, generator.ErrFeatureDisabled) {
		t.Fatalf("direct BuildOpenAPI error = %v, want ErrFeatureDisabled", err)
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertTypeUsage(t *testing.T, plan *generator.PackagePlan, name string, want generator.Usage) {
	t.Helper()
	for _, typ := range plan.Types {
		if typ.Name == name {
			if typ.DirectUsage != want {
				t.Fatalf("%s usage = %v, want %v", name, typ.DirectUsage, want)
			}
			return
		}
	}
	t.Fatalf("type %s not found", name)
}
