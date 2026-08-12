package generator_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cbcg "github.com/shibukawa/tinybind-go/configbind/codegen"
	"github.com/shibukawa/tinybind-go/generator"
)

func TestGenerateConfigBindFromFixture(t *testing.T) {
	dir := filepath.Join("..", "internal", "configbindfixture")
	g := generator.New(generator.DefaultOptions())
	outDir := t.TempDir()
	path, err := g.GenerateConfigBind(dir, outDir, "configbind_gen.go")
	if err != nil {
		t.Fatal(err)
	}
	if path == "" {
		t.Fatal("expected generated path")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"Register[WebServerConfig]",
		`"webserver.port"`,
		`"webserver.tls.enabled"`,
		`Opt: "port,p"`,
		`Env: "TLS_CERT_FILE"`,
		`Scaffold: []configbind.ScaffoldField`,
		// time.Duration must not degrade to an int field.
		`Kind: configbind.ScaffoldDuration`,
		`time.ParseDuration`,
		`DependsOn: map[string][]configbind.Dependency{`,
		`{{Key: "webserver.tls.enabled"}}`,
		`Key: "tls.cert_path"`,
		`Env: "TLS_CERT_FILE"`,
		"applyWebServerConfigDefinition0",
		"RegisterSubCommand[MigrateOptions]",
		`Name:     "migrate"`,
		"configbind.PositionalRequired",
		"configbind.PositionalOptional",
		"configbind.PositionalRest",
		"cliparser.FieldMeta",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in\n%s", want, text)
		}
	}
	if strings.Contains(text, "reflect.") {
		t.Fatal("must not use reflect")
	}
}

func TestGenerateCommandConfigBind(t *testing.T) {
	dir := filepath.Join("..", "internal", "configbindfixture")
	out := t.TempDir()
	set := generator.MustCommandSet(generator.GenerateCommand(generator.DefaultOptions()))
	code := set.Run(context.Background(), []string{
		"generate", "-dir", dir,
		"-out", out,
		"-openapi=false",
	}, generator.CommandIO{Stdout: os.Stdout, Stderr: os.Stderr})
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	gen := filepath.Join(out, "configbind_gen.go")
	data, err := os.ReadFile(gen)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "WebServerConfig") {
		t.Fatalf("unexpected gen:\n%s", data)
	}
}

func TestGeneratePackageReturnsConfigBindArtifact(t *testing.T) {
	dir := filepath.Join("..", "internal", "configbindfixture")
	out := t.TempDir()
	result, err := generator.New(generator.DefaultOptions()).GeneratePackage(context.Background(), generator.GenerateRequest{
		Dir: dir, Out: out, OpenAPI: false, ConfigBindName: "framework_config_gen.go",
	})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(result.ConfigBindPath) != "framework_config_gen.go" {
		t.Fatalf("ConfigBindPath=%q paths=%v", result.ConfigBindPath, result.Paths())
	}
	if _, err := os.Stat(result.ConfigBindPath); err != nil {
		t.Fatal(err)
	}
}

// writeConfigBindModule writes a temp module whose package binds one config
// struct, and returns its directory.
func writeConfigBindModule(t *testing.T, source string) string {
	t.Helper()
	dir := t.TempDir()
	writeTempModule(t, dir)
	src := `package sample

import "github.com/shibukawa/tinybind-go/configbind"

func Register() *AppConfig { return configbind.Bind[AppConfig]("app") }

` + source
	if err := os.WriteFile(filepath.Join(dir, "types.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	return dir
}

func TestAnalyzeConfigBindTableArray(t *testing.T) {
	dir := writeConfigBindModule(t, `
type AppConfig struct {
	Name    string
	Routes  []RouteConfig `+"`help:\"static routes\"`"+`
}

type RouteConfig struct {
	Path    string
	Listing bool `+"`default:\"false\"`"+`
	Rewrite RewriteConfig
}

type RewriteConfig struct {
	From string
	To   string
}
`)
	_, specs, err := generator.AnalyzeConfigBind(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 1 {
		t.Fatalf("specs=%+v", specs)
	}
	var routes *cbcg.Field
	for i := range specs[0].Fields {
		if specs[0].Fields[i].GoName == "Routes" {
			routes = &specs[0].Fields[i]
		}
	}
	if routes == nil {
		t.Fatalf("Routes field not discovered: %+v", specs[0].Fields)
	}
	if routes.Kind != cbcg.FieldStructSlice {
		t.Fatalf("Routes kind=%v", routes.Kind)
	}
	if routes.Key != "routes" || routes.ElemType != "RouteConfig" {
		t.Fatalf("Routes=%+v", routes)
	}
	if len(routes.Nested) != 3 {
		t.Fatalf("element fields=%+v", routes.Nested)
	}
	if routes.Nested[2].GoName != "Rewrite" || len(routes.Nested[2].Nested) != 2 {
		t.Fatalf("nested struct inside the element=%+v", routes.Nested[2])
	}
}

func TestAnalyzeConfigBindRejectsUnsupportedSliceShapes(t *testing.T) {
	cases := []struct {
		name    string
		source  string
		wantSub string
	}{
		{
			name: "pointer_elements",
			source: `
type AppConfig struct {
	Routes []*RouteConfig
}

type RouteConfig struct{ Path string }
`,
			wantSub: "use []RouteConfig instead of []*RouteConfig",
		},
		{
			name: "recursive_element",
			source: `
type AppConfig struct {
	Routes []RouteConfig
}

type RouteConfig struct {
	Path     string
	Children []RouteConfig
}
`,
			wantSub: "recursive config struct RouteConfig",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := writeConfigBindModule(t, tc.source)
			_, _, err := generator.AnalyzeConfigBind(dir)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("error %q does not contain %q", err, tc.wantSub)
			}
		})
	}
}
