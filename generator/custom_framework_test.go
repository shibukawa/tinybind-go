package generator_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

// customFrameworkOptions mirrors what a framework generator command builds
// once: its own call vocabulary plus its own published API shapes.
func customFrameworkOptions(t *testing.T) generator.Options {
	t.Helper()
	registry := generator.NewCallRegistry()
	err := registry.Register(
		generator.RequestBindCall(
			generator.Function("tempmod/pw", "Parse"),
			generator.GenericType("request", 0),
		),
		generator.ResponseWriteCall(
			generator.Function("tempmod/pw", "WriteAPI"),
			generator.GenericType("response", 0),
		),
		generator.ConfigBindCall(
			generator.Function("tempmod/pw", "RegisterConfig"),
			generator.GenericType("config", 0), generator.Argument("prefix", 0),
		),
		generator.ConfigSubCommandCall(
			generator.Function("tempmod/pw", "SubCommand"),
			generator.GenericType("config", 0), generator.Argument("name", 0), generator.Argument("help", 1),
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	options, err := registry.Options(generator.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	options.HTMLTemplatePattern = "*.pw.html"
	options.SQLTemplatePattern = "*.pw.sql"
	options.SQLContextOnlyAPI = true
	options.SQLExecutorResolver = &generator.SymbolPattern{PackagePath: "tempmod/pw", Name: "SQLExecutor"}
	return options
}

// copyCustomFrameworkFixture materializes testdata/custom_framework in a temp
// module so the generated artifacts can be compiled and tested.
func copyCustomFrameworkFixture(t *testing.T) (root, fixture string) {
	t.Helper()
	root = t.TempDir()
	writeTempModule(t, root)
	source := filepath.Join("..", "testdata", "custom_framework")
	for _, pkg := range []string{"pw", "fixture"} {
		if err := os.MkdirAll(filepath.Join(root, pkg), 0o755); err != nil {
			t.Fatal(err)
		}
		entries, err := os.ReadDir(filepath.Join(source, pkg))
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			content, err := os.ReadFile(filepath.Join(source, pkg, entry.Name()))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, pkg, entry.Name()), content, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	tidyTempModule(t, root)
	return root, filepath.Join(root, "fixture")
}

func TestCustomFrameworkGenerationProfile(t *testing.T) {
	root, fixture := copyCustomFrameworkFixture(t)
	runner := generator.New(customFrameworkOptions(t))

	artifacts, err := runner.GenerateArtifacts(context.Background(), generator.GenerateRequest{Dir: fixture, OpenAPI: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) == 0 {
		t.Fatal("no artifacts generated")
	}

	// Nothing may be written by GenerateArtifacts itself.
	entries, err := os.ReadDir(fixture)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), "_pw_gen.go") {
			t.Fatalf("GenerateArtifacts wrote %s", entry.Name())
		}
	}

	byKind := map[generator.ArtifactKind][]generator.Artifact{}
	for _, artifact := range artifacts {
		byKind[artifact.Kind] = append(byKind[artifact.Kind], artifact)
	}
	for _, kind := range []generator.ArtifactKind{
		generator.ArtifactHTMLTemplate,
		generator.ArtifactSQLTemplate,
		generator.ArtifactBinding,
		generator.ArtifactConfigBind,
		generator.ArtifactOpenAPI,
	} {
		if len(byKind[kind]) == 0 {
			t.Fatalf("no %s artifact generated", kind)
		}
	}

	// Custom-suffix sources are discovered directly and own their artifacts.
	wantOwners := map[generator.ArtifactKind]string{
		generator.ArtifactHTMLTemplate: "page.pw.html",
		generator.ArtifactSQLTemplate:  "users.pw.sql",
		generator.ArtifactConfigBind:   "config.go",
	}
	for kind, want := range wantOwners {
		got := byKind[kind][0]
		if filepath.Base(got.SourcePath) != want {
			t.Fatalf("%s artifact owner = %q, want %q", kind, got.SourcePath, want)
		}
		if got.OutputBase != strings.SplitN(want, ".", 2)[0] {
			t.Fatalf("%s artifact base = %q", kind, got.OutputBase)
		}
	}
	if base := byKind[generator.ArtifactBinding][0].OutputBase; base != "handler" {
		t.Fatalf("binding artifact base = %q, want handler", base)
	}

	html := byKind[generator.ArtifactHTMLTemplate][0].GoSource
	for _, want := range []string{
		"type UserPageParams struct {",
		"func UserPage(params UserPageParams) htmlbind.Fragment",
	} {
		if !bytes.Contains(html, []byte(want)) {
			t.Fatalf("HTML artifact lacks %q:\n%s", want, html)
		}
	}
	if bytes.Contains(html, []byte("http.ResponseWriter")) {
		t.Fatalf("HTML artifact is not HTTP independent:\n%s", html)
	}

	sql := byKind[generator.ArtifactSQLTemplate][0].GoSource
	for _, want := range []string{
		"func FindUser(ctx context.Context, id int) (UserRow, error)",
		"func BuildFindUser(id int) (_tinybindsql.Statement, error)",
		"_tinybindresolver.SQLExecutor(ctx)",
	} {
		if !bytes.Contains(sql, []byte(want)) {
			t.Fatalf("SQL artifact lacks %q:\n%s", want, sql)
		}
	}
	if bytes.Contains(sql, []byte("func FindUserContext(")) {
		t.Fatalf("SQL artifact still exposes a Context wrapper:\n%s", sql)
	}

	config := byKind[generator.ArtifactConfigBind][0].GoSource
	for _, want := range []string{`"generate-config"`, `"write merged configuration scaffolds"`} {
		if !bytes.Contains(config, []byte(want)) {
			t.Fatalf("configbind artifact lacks %q:\n%s", want, config)
		}
	}

	writeArtifacts(t, fixture, artifacts)
	runModuleTests(t, root)
}

// writeArtifacts applies the framework's own {source-base}_pw_gen.go naming.
func writeArtifacts(t *testing.T, dir string, artifacts []generator.Artifact) {
	t.Helper()
	for _, artifact := range artifacts {
		name := artifact.OutputBase + "_pw_gen.go"
		if err := os.WriteFile(filepath.Join(dir, name), artifact.GoSource, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func runModuleTests(t *testing.T, root string) {
	t.Helper()
	command := exec.Command("go", "test", "./...")
	command.Dir = root
	command.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("go test ./... failed: %v\n%s", err, output)
	}
}

func TestCustomFrameworkArtifactsAreDeterministic(t *testing.T) {
	_, fixture := copyCustomFrameworkFixture(t)
	runner := generator.New(customFrameworkOptions(t))
	request := generator.GenerateRequest{Dir: fixture, OpenAPI: true}

	first, err := runner.GenerateArtifacts(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := runner.GenerateArtifacts(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != len(second) {
		t.Fatalf("artifact count changed: %d then %d", len(first), len(second))
	}
	for i := range first {
		if first[i].SourcePath != second[i].SourcePath || first[i].Kind != second[i].Kind {
			t.Fatalf("artifact %d identity changed: %+v vs %+v", i, first[i], second[i])
		}
		if !bytes.Equal(first[i].GoSource, second[i].GoSource) {
			t.Fatalf("artifact %s is not byte-identical across runs", first[i].OutputBase)
		}
	}
}

func TestCustomSuffixDiagnosticsReportTheRealSource(t *testing.T) {
	_, fixture := copyCustomFrameworkFixture(t)
	broken := filepath.Join(fixture, "broken.pw.html")
	writeTestFile(t, broken, "package fixture\n\nexport component Broken(): html {\n<p>\n{missing}\n</p>\n}\n")

	runner := generator.New(customFrameworkOptions(t))
	_, err := runner.GenerateArtifacts(context.Background(), generator.GenerateRequest{Dir: fixture})
	if err == nil {
		t.Fatal("expected a diagnostic for the broken template")
	}
	if !strings.Contains(err.Error(), "broken.pw.html:5:2:") {
		t.Fatalf("diagnostic = %v, want the real .pw.html path, line, and column", err)
	}
	if strings.Contains(err.Error(), ".tb.html") {
		t.Fatalf("diagnostic mentions the default suffix: %v", err)
	}
}

func TestCustomFrameworkPackageAggregationSharesTheAPIShapes(t *testing.T) {
	root, fixture := copyCustomFrameworkFixture(t)
	runner := generator.New(customFrameworkOptions(t))

	result, err := runner.GeneratePackage(context.Background(), generator.GenerateRequest{
		Dir:           fixture,
		Name:          "handler_pw_gen.go",
		TemplatesName: "templates_pw_gen.go",
		OpenAPIName:   "openapi_pw_gen.go",
		OpenAPI:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Paths()) == 0 {
		t.Fatal("no files written")
	}
	templates, err := os.ReadFile(result.TemplatesPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"func UserPage(params UserPageParams) htmlbind.Fragment",
		"func FindUser(ctx context.Context, id int) (UserRow, error)",
	} {
		if !bytes.Contains(templates, []byte(want)) {
			t.Fatalf("aggregated templates lack %q:\n%s", want, templates)
		}
	}
	if bytes.Contains(templates, []byte("http.ResponseWriter")) {
		t.Fatalf("aggregated templates are not HTTP independent:\n%s", templates)
	}
	runModuleTests(t, root)
}

func TestGenerateArtifactsUsesDefaultTemplateSuffixes(t *testing.T) {
	root, fixture := newFrameworkModule(t, map[string]string{
		"doc.go":       "package fixture\n",
		"page.tb.html": "package fixture\n\nexport component Page(name: string): html {\n<p>{name}</p>\n}\n",
	})
	artifacts, err := generator.New(generator.DefaultOptions()).GenerateArtifacts(
		context.Background(),
		generator.GenerateRequest{Dir: fixture},
	)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, artifact := range artifacts {
		if artifact.Kind != generator.ArtifactHTMLTemplate {
			continue
		}
		found = true
		if filepath.Base(artifact.SourcePath) != "page.tb.html" {
			t.Fatalf("owner = %q", artifact.SourcePath)
		}
		if !bytes.Contains(artifact.GoSource, []byte("func Page(params PageParams) htmlbind.Fragment")) {
			t.Fatalf("generated shape changed:\n%s", artifact.GoSource)
		}
	}
	if !found {
		t.Fatal("default .tb.html suffix was not discovered")
	}
	writeArtifacts(t, fixture, artifacts)
	compileModule(t, root)
}
