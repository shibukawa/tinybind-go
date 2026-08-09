package generator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	var joined string
	for _, d := range result.Diagnostics {
		if d.File == "" || d.Line == 0 {
			t.Errorf("diagnostic has no position: %+v", d)
		}
		joined += d.Message + "\n"
	}
	for _, want := range []string{"unknown_call", "inherited", "remedy:"} {
		if !strings.Contains(joined, want) {
			t.Errorf("diagnostics missing %q:\n%s", want, joined)
		}
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
