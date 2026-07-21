package generator_test

import (
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
		"RegisterType[WebServerConfig]",
		`"webserver.port"`,
		`"webserver.tls.enabled"`,
		`Opt: "port,p"`,
		"applyWebServerConfig",
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

func TestRunTinybindGenConfigBind(t *testing.T) {
	dir := filepath.Join("..", "internal", "configbindfixture")
	out := t.TempDir()
	code := generator.Run([]string{
		"-dir", dir,
		"-out", out,
		"-openapi=false",
	}, os.Stdout, os.Stderr, generator.DefaultOptions())
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
