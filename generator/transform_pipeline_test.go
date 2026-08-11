package generator

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// skipWithoutToolchain skips a test that shells out to the Go toolchain. It is
// the in-package twin of the one in testmod_test.go, which the external test
// package uses; the two exist because these compile checks live beside the
// transform they exercise.
func skipWithoutToolchain(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("short mode: this test runs the Go toolchain against a temp module")
	}
}

func generateInto(t *testing.T, fixture string, transform *TransformOptions) (GenerateResult, string, error) {
	t.Helper()
	out := t.TempDir()
	options := DefaultOptions()
	options.Transform = transform
	result, err := New(options).GeneratePackage(context.Background(), GenerateRequest{
		Dir: filepath.Join("..", "testdata", fixture),
		Out: out,
	})
	return result, out, err
}

// The feature is opt-in, so a run that does not ask for a backend has to look
// exactly like one from before the transform existed.
func TestNoBackendSelectedWritesNoTransportFile(t *testing.T) {
	result, out, err := generateInto(t, "transform_rewrite", nil)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if result.TransportPath != "" {
		t.Errorf("TransportPath = %q with no backend selected", result.TransportPath)
	}
	entries, _ := os.ReadDir(out)
	for _, entry := range entries {
		if strings.Contains(entry.Name(), "transport") {
			t.Errorf("wrote %s with no backend selected", entry.Name())
		}
	}
}

func TestSelectingTheBackendWritesTheTransportFile(t *testing.T) {
	transform := DefaultTransformOptions()
	result, _, err := generateInto(t, "transform_rewrite", &transform)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if result.TransportPath == "" {
		t.Fatal("no transport file was written")
	}
	if base := filepath.Base(result.TransportPath); base != defaultTransportOut {
		t.Errorf("transport file = %q, want %q", base, defaultTransportOut)
	}
	source, err := os.ReadFile(result.TransportPath)
	if err != nil {
		t.Fatalf("read transport file: %v", err)
	}
	for _, want := range []string{
		"//go:build fasthttp",
		"func createUser(ctx *fasthttp.RequestCtx)",
		`httpbind "github.com/shibukawa/tinybind-go/fasthttpbind"`,
	} {
		if !strings.Contains(string(source), want) {
			t.Errorf("transport file missing %q:\n%s", want, source)
		}
	}
	// It joins the run's outputs, so the stamp and the caller's file list see it.
	if !contains(result.Paths(), result.TransportPath) {
		t.Error("the transport file is missing from the run's paths")
	}
}

// With no adapter there is nowhere for a refused handler to go, so emitting
// the rest would leave a package that silently serves fewer routes.
func TestARefusalStopsGeneration(t *testing.T) {
	transform := DefaultTransformOptions()
	_, out, err := generateInto(t, "transform_eligibility", &transform)
	if err == nil {
		t.Fatal("generation succeeded on a package with untransformable handlers")
	}
	for _, want := range []string{"unknownCallHandler", "not transformable", "remedy:"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error missing %q:\n%v", want, err)
		}
	}
	entries, _ := os.ReadDir(out)
	for _, entry := range entries {
		if strings.Contains(entry.Name(), "transport") {
			t.Errorf("wrote %s despite refusing", entry.Name())
		}
	}
}

// Adoption is all-or-nothing, so an application has to be able to see the whole
// cost before committing rather than one refusal per build afterwards.
func TestReportOnlyListsRefusalsAndWritesNothing(t *testing.T) {
	transform := DefaultTransformOptions()
	transform.ReportOnly = true
	result, out, err := generateInto(t, "transform_eligibility", &transform)
	if err != nil {
		t.Fatalf("report-only run failed: %v", err)
	}
	if len(result.Diagnostics) < 7 {
		t.Errorf("reported %d diagnostics, expected one per refusal", len(result.Diagnostics))
	}
	// The classification goes in Reason, where a consumer reads it, and the
	// prose carries the occurrence and the remedy.
	kinds := map[string]bool{}
	var joined string
	for _, d := range result.Diagnostics {
		if d.File == "" || d.Line == 0 {
			t.Errorf("diagnostic has no position: %+v", d)
		}
		kinds[d.Reason] = true
		joined += d.Message + "\n"
	}
	for _, want := range []string{"unknown_call", "inherited", "type_assertion", "escapes"} {
		if !kinds[want] {
			t.Errorf("no diagnostic classified %q; saw %v", want, kinds)
		}
	}
	if !strings.Contains(joined, "remedy:") {
		t.Errorf("diagnostics carry no remedy:\n%s", joined)
	}
	if entries, _ := os.ReadDir(out); len(entries) != 0 {
		t.Errorf("report-only wrote %d files", len(entries))
	}
}

func TestReportOnlyOnACleanPackageSaysNothing(t *testing.T) {
	transform := DefaultTransformOptions()
	transform.ReportOnly = true
	result, out, err := generateInto(t, "transform_rewrite", &transform)
	if err != nil {
		t.Fatalf("report-only run failed: %v", err)
	}
	if len(result.Diagnostics) != 0 {
		t.Errorf("a fully transformable package reported %d diagnostics", len(result.Diagnostics))
	}
	if entries, _ := os.ReadDir(out); len(entries) != 0 {
		t.Errorf("report-only wrote %d files", len(entries))
	}
}

// The whole loop: one authored package generates two builds, and each compiles
// on its own. Everything else in this file is a claim about which file was
// written; this is the one that would catch two halves that do not fit.
func TestBothTagConfigurationsCompile(t *testing.T) {
	skipWithoutToolchain(t)
	transform := DefaultTransformOptions()
	result, out, err := generateInto(t, "transform_rewrite", &transform)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if result.FastBindersPath == "" {
		t.Fatal("no backend binder file was written")
	}

	// The generated files join the authored package, which is where they are
	// meant to live; a temp directory could not resolve the package's own types.
	const dir = "testdata/transform_rewrite"
	entries, err := os.ReadDir(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	for _, entry := range entries {
		source, err := os.ReadFile(filepath.Join(out, entry.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name(), err)
		}
		path := filepath.Join("..", dir, entry.Name())
		if err := os.WriteFile(path, source, 0o644); err != nil {
			t.Fatalf("place %s: %v", entry.Name(), err)
		}
		t.Cleanup(func() { os.Remove(path) })
	}

	for _, tags := range []string{"fasthttp", ""} {
		args := []string{"build", "-o", os.DevNull}
		if tags != "" {
			args = append(args, "-tags", tags)
		}
		cmd := exec.Command("go", append(args, "./"+dir)...)
		cmd.Dir = ".."
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("build with tags %q failed: %v\n%s", tags, err, output)
		}
	}
}

func TestRouteRegistrationIsGenerated(t *testing.T) {
	transform := DefaultTransformOptions()
	result, _, err := generateInto(t, "transform_rewrite", &transform)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if result.RoutesPath == "" {
		t.Fatal("no route registration was written")
	}
	source, err := os.ReadFile(result.RoutesPath)
	if err != nil {
		t.Fatalf("read routes: %v", err)
	}
	for _, want := range []string{
		"//go:build fasthttp",
		`router "github.com/shibukawa/tinygodriver/fasthttprouter"`,
		"func RegisterRoutes(r *router.Router)",
		// A named parameter is spelled the same by both routers, so the
		// discovered pattern carries over untouched.
		`r.Handle("GET", "/users/{id}", cancelAware)`,
		`r.Handle("POST", "/users", createUser)`,
		// The catch-all is the one segment shape that moves.
		`r.Handle("GET", "/files/{rest:*}", cancelAware)`,
	} {
		if !strings.Contains(string(source), want) {
			t.Errorf("route registration missing %q:\n%s", want, source)
		}
	}
}

// Which router an application depends on is its choice, not this module's.
func TestRouterTargetIsConfigurable(t *testing.T) {
	transform := DefaultTransformOptions()
	transform.Router = RouterTarget{
		Import:         "example.com/fw/mux",
		Qualifier:      "mux",
		Type:           "mux.Router",
		RegisterFunc:   "Wire",
		CatchAllSuffix: ":*",
	}
	result, _, err := generateInto(t, "transform_rewrite", &transform)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	source, err := os.ReadFile(result.RoutesPath)
	if err != nil {
		t.Fatalf("read routes: %v", err)
	}
	for _, want := range []string{`mux "example.com/fw/mux"`, "func Wire(r mux.Router)"} {
		if !strings.Contains(string(source), want) {
			t.Errorf("missing %q:\n%s", want, source)
		}
	}
}

// A router with no catch-all spelling gets an error rather than a guess.
func TestCatchAllWithoutAnEquivalentIsRefused(t *testing.T) {
	transform := DefaultTransformOptions()
	transform.Router.CatchAllSuffix = ""
	if _, _, err := generateInto(t, "transform_rewrite", &transform); err == nil ||
		!strings.Contains(err.Error(), "catch-all") {
		t.Errorf("error = %v, want it to name the catch-all pattern", err)
	}
}
