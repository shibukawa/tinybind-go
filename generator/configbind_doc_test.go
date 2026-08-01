package generator_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

const configDocFixture = `package config

import "github.com/shibukawa/tinybind-go/configbind"

// ServerConfig configures the public HTTP listener.
type ServerConfig struct {
	// Port is the HTTP listen port.
	Port int ` + "`default:\"8080\"`" + `
	Host string // Host is the listen address.
	// Timeout is ignored because the tag wins.
	Timeout int ` + "`help:\"request timeout seconds\"`" + `
	//go:generate echo directive-only
	Quiet bool
	TLS   TLSConfig
}

// TLSConfig configures transport security.
type TLSConfig struct {
	// Enabled turns TLS on.
	//
	// This second paragraph must not reach the help text.
	Enabled bool
}

// MigrateOptions runs pending database migrations.
type MigrateOptions struct {
	// Path is the migration directory.
	Path string ` + "`arg:\"required\"`" + `
}

// Bind registers the server configuration.
func Bind() *ServerConfig { return configbind.Bind[ServerConfig]("server") }

// Migrate registers the migrate subcommand with no inline help text.
func Migrate() *MigrateOptions { return configbind.SubCommand[MigrateOptions]("migrate", "") }
`

// writeConfigDocFixture materializes the fixture in a temp module so backfill can
// rewrite it without touching repository sources.
func writeConfigDocFixture(t *testing.T) (dir, source string) {
	t.Helper()
	root := t.TempDir()
	writeTempModule(t, root)
	dir = filepath.Join(root, "config")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	source = filepath.Join(dir, "config.go")
	if err := os.WriteFile(source, []byte(configDocFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, root)
	return dir, source
}

func TestConfigBindGodocSeedsHelpAndBackfillsTags(t *testing.T) {
	dir, source := writeConfigDocFixture(t)

	_, specs, err := generator.AnalyzeConfigBind(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 2 {
		t.Fatalf("specs=%d want 2", len(specs))
	}
	var server, migrate = -1, -1
	for i, spec := range specs {
		switch spec.TypeName {
		case "ServerConfig":
			server = i
		case "MigrateOptions":
			migrate = i
		}
	}
	if server < 0 || migrate < 0 {
		t.Fatalf("missing specs: %+v", specs)
	}

	if got, want := specs[server].Doc, "ServerConfig configures the public HTTP listener"; got != want {
		t.Fatalf("ServerConfig Doc=%q want %q", got, want)
	}
	if got, want := specs[migrate].Doc, "MigrateOptions runs pending database migrations"; got != want {
		t.Fatalf("MigrateOptions Doc=%q want %q", got, want)
	}

	help := map[string]string{}
	for _, field := range specs[server].Fields {
		help[field.Key] = field.Help
		for _, nested := range field.Nested {
			help[field.Key+"."+nested.Key] = nested.Help
		}
	}
	for key, want := range map[string]string{
		"port":        "Port is the HTTP listen port",
		"host":        "Host is the listen address",
		"timeout":     "request timeout seconds",
		"quiet":       "",
		"tls.enabled": "Enabled turns TLS on",
	} {
		if got := help[key]; got != want {
			t.Fatalf("help[%q]=%q want %q", key, got, want)
		}
	}

	rewritten, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	text := string(rewritten)
	for _, want := range []string{
		`default:"8080" help:"Port is the HTTP listen port"`,
		"Host string `help:\"Host is the listen address\"`",
		`help:"request timeout seconds"`,
		`help:"Enabled turns TLS on"`,
		"Path string `arg:\"required\" help:\"Path is the migration directory\"`",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in rewritten source:\n%s", want, text)
		}
	}
	if strings.Contains(text, `help:"Timeout is ignored`) {
		t.Fatalf("existing help tag was overwritten:\n%s", text)
	}
	if strings.Contains(text, "Quiet bool `") {
		t.Fatalf("directive-only comment produced a tag:\n%s", text)
	}
	if strings.Contains(text, "second paragraph") && strings.Contains(text, `help:"Enabled turns TLS on. This`) {
		t.Fatalf("second paragraph leaked into help:\n%s", text)
	}

	// A second run must find every tag in place and change nothing.
	if _, _, err := generator.AnalyzeConfigBind(dir); err != nil {
		t.Fatal(err)
	}
	again, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != text {
		t.Fatalf("backfill is not idempotent:\n--- first ---\n%s\n--- second ---\n%s", text, again)
	}
}

func TestConfigBindHelpBackfillCanBeDisabled(t *testing.T) {
	dir, source := writeConfigDocFixture(t)
	options := generator.DefaultOptions()
	options.DisableFeatures = append(options.DisableFeatures, generator.FeatureHelpBackfill)

	_, specs, err := generator.AnalyzeConfigBindWithOptions(dir, options)
	if err != nil {
		t.Fatal(err)
	}
	for _, spec := range specs {
		if spec.TypeName != "ServerConfig" {
			continue
		}
		for _, field := range spec.Fields {
			if field.Key == "port" && field.Help != "Port is the HTTP listen port" {
				t.Fatalf("port help=%q; godoc must still seed help when backfill is off", field.Help)
			}
		}
	}
	unchanged, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if string(unchanged) != configDocFixture {
		t.Fatalf("source was rewritten with backfill disabled:\n%s", unchanged)
	}
}

func TestConfigBindGeneratesDocAndSubcommandHelpFallback(t *testing.T) {
	dir, _ := writeConfigDocFixture(t)
	out := t.TempDir()
	path, err := generator.New(generator.DefaultOptions()).GenerateConfigBind(dir, out, "configbind_gen.go")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Field alignment comes from gofmt, so match on the key and value only.
	text := strings.Join(strings.Fields(string(data)), " ")
	for _, want := range []string{
		`Doc: "ServerConfig configures the public HTTP listener"`,
		`Help: "Port is the HTTP listen port"`,
		`Help: "MigrateOptions runs pending database migrations"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in generated source:\n%s", want, text)
		}
	}
}
