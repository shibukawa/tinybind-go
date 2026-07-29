package configbindfixture_test

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/configbind"
	"github.com/shibukawa/tinybind-go/internal/configbindfixture"
	"github.com/shibukawa/tinybind-go/minitoml"
)

func TestGeneratedScaffolds(t *testing.T) {
	tomlText, err := configbind.ScaffoldTOML()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(tomlText, "[webserver]\n") {
		t.Fatalf("TOML scaffold=%q", tomlText)
	}
	// An array of tables becomes its own block, after the prefix's own keys.
	if !strings.Contains(tomlText, "\n[[webserver.routes]]\n# URL path prefix\npath = \"\"\n") {
		t.Fatalf("TOML scaffold=%q", tomlText)
	}
	if strings.Index(tomlText, "port = 8080") > strings.Index(tomlText, "[[webserver.routes]]") {
		t.Fatalf("scalar keys must precede the array block:\n%s", tomlText)
	}
	// The scaffold has to be readable by the parser it is written for.
	if _, err := minitoml.ParseString(tomlText); err != nil {
		t.Fatalf("scaffold does not parse: %v\n%s", err, tomlText)
	}
	envText, err := configbind.ScaffoldEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(envText, "TLS_CERT_FILE=\"\"\n") {
		t.Fatalf("env scaffold=%q", envText)
	}
}

func TestGeneratedLoadPrecedence(t *testing.T) {
	configbind.ResetTargets()
	cfg := configbindfixture.Register()

	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "config.toml")
	body := `
[webserver]
port = 1
host = "from-toml"
cors_origins = ["a.example", "b.example"]
max_request_body = 9223372036854775807
tls.enabled = true
tls.cert_path = "toml.crt"
`
	if err := os.WriteFile(tomlPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	// Env names follow CLI long opts: port -> PORT; host -> WEBSERVER_HOST; etc.
	environ := []string{
		"WEBSERVER_HOST=from-env",
		"TLS_CERT_FILE=env.crt",
		"WEBSERVER_TLS_CERT_PATH=ignored.crt",
		"PORT=2",
	}
	// CLI wins port; also sets cors
	args := []string{
		"--config-path", tomlPath,
		"--port", "99",
		"--webserver-cors_origins", "cli.example",
	}

	res, err := configbind.Load(configbind.LoadOptions{
		Vendor:  "acme",
		Tool:    "demo",
		Environ: environ,
		Args:    args,
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !res.FoundFile || res.ConfigPath == "" {
		t.Fatalf("expected config file, got %+v", res)
	}
	if cfg.Port != 99 {
		t.Fatalf("Port=%d want 99 (CLI over env/toml)", cfg.Port)
	}
	if cfg.Host != "from-env" {
		t.Fatalf("Host=%q want from-env", cfg.Host)
	}
	// A sized integer keeps its width: an int64 field must survive a value no
	// int32 could hold, and must not be narrowed on the way in.
	if cfg.MaxRequestBody != math.MaxInt64 {
		t.Fatalf("MaxRequestBody=%d want %d", cfg.MaxRequestBody, int64(math.MaxInt64))
	}
	if cfg.TLS.CertPath != "env.crt" {
		t.Fatalf("CertPath=%q want env.crt", cfg.TLS.CertPath)
	}
	if !cfg.TLS.Enabled {
		t.Fatal("TLS.Enabled want true from toml")
	}
	if len(cfg.CorsOrigins) != 1 || cfg.CorsOrigins[0] != "cli.example" {
		t.Fatalf("CorsOrigins=%v", cfg.CorsOrigins)
	}

	// Provenance-ish checks on overlay places
	if e, ok := res.Overlay.Get("webserver.port"); !ok || e.Place != configbind.PlaceCLI {
		t.Fatalf("port place=%v ok=%v", e, ok)
	}
	if e, ok := res.Overlay.Get("webserver.host"); !ok || e.Place != configbind.PlaceEnv {
		t.Fatalf("host place=%v ok=%v", e, ok)
	}
	if e, ok := res.Overlay.Get("webserver.tls.enabled"); !ok || e.Place != configbind.PlaceFile {
		t.Fatalf("tls.enabled place=%v ok=%v", e, ok)
	}
}

func TestGeneratedEnvNameFromLongOpt(t *testing.T) {
	// opt:"port,p" => long "port" => PORT
	if configbind.EnvName("port") != "PORT" {
		t.Fatal(configbind.EnvName("port"))
	}
	// default long for host is webserver-host
	if configbind.EnvName("webserver-host") != "WEBSERVER_HOST" {
		t.Fatal(configbind.EnvName("webserver-host"))
	}
	if configbind.EnvName("webserver-tls-cert_path") != "WEBSERVER_TLS_CERT_PATH" {
		t.Fatal(configbind.EnvName("webserver-tls-cert_path"))
	}
}

func TestGeneratedSubCommand(t *testing.T) {
	configbind.ResetTargets()
	previousArgs := os.Args
	os.Args = []string{"demo", "migrate", "./migrations", "--dry_run", "release", "one", "two"}
	t.Cleanup(func() { os.Args = previousArgs })

	options := configbindfixture.RegisterMigrate()
	if options == nil {
		t.Fatal("selected generated subcommand is nil")
	}
	_, err := configbind.Load(configbind.LoadOptions{
		Args:    os.Args[1:],
		Environ: []string{"DRY_RUN=false"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if options.Path != "./migrations" || options.Label != "release" || !options.DryRun {
		t.Fatalf("options=%+v", options)
	}
	if got := options.Extra; len(got) != 2 || got[0] != "one" || got[1] != "two" {
		t.Fatalf("Extra=%v", got)
	}
}

func TestGeneratedLoadTableArray(t *testing.T) {
	configbind.ResetTargets()
	cfg := configbindfixture.Register()

	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "config.toml")
	body := `
[webserver]
port = 1

[[webserver.routes]]
path = "/"
dir = "./public"

[[webserver.routes]]
path = "/files"
dir = "./files"
listing = true
`
	if err := os.WriteFile(tomlPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := configbind.Load(configbind.LoadOptions{
		Vendor:  "acme",
		Tool:    "demo",
		Environ: []string{},
		Args:    []string{"--config-path", tomlPath},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Routes) != 2 {
		t.Fatalf("Routes=%+v", cfg.Routes)
	}
	if cfg.Routes[0].Path != "/" || cfg.Routes[0].Dir != "./public" || cfg.Routes[0].Listing {
		t.Fatalf("Routes[0]=%+v", cfg.Routes[0])
	}
	if cfg.Routes[1].Path != "/files" || !cfg.Routes[1].Listing {
		t.Fatalf("Routes[1]=%+v", cfg.Routes[1])
	}

	entry, ok := res.Overlay.Get("webserver.routes")
	if !ok || !entry.IsTables || entry.Place != configbind.PlaceFile {
		t.Fatalf("routes entry=%+v ok=%v", entry, ok)
	}
	tables, ok := res.Overlay.GetTables("webserver.routes")
	if !ok || len(tables) != 2 {
		t.Fatalf("GetTables=%v ok=%v", tables, ok)
	}
	// Element keys are relative to the array key.
	if v, ok := tables[1].GetString("path"); !ok || v != "/files" {
		t.Fatalf("tables[1].path=%q ok=%v", v, ok)
	}
}

func TestGeneratedLoadRejectsScalarForTableArray(t *testing.T) {
	configbind.ResetTargets()
	configbindfixture.Register()

	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(tomlPath, []byte("[webserver]\nroutes = 3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := configbind.Load(configbind.LoadOptions{
		Vendor:  "acme",
		Tool:    "demo",
		Environ: []string{},
		Args:    []string{"--config-path", tomlPath},
	})
	if err == nil {
		t.Fatal("expected an error for a scalar in place of an array of tables")
	}
	if !strings.Contains(err.Error(), "webserver.routes") {
		t.Fatalf("error %q must name the key", err)
	}
}
