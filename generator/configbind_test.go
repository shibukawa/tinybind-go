package generator_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
		`DependsOn: map[string]string{`,
		`"webserver.tls.cert_path": "webserver.tls.enabled"`,
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
