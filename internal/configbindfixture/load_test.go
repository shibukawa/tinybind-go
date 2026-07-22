package configbindfixture_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/configbind"
	"github.com/shibukawa/tinybind-go/internal/configbindfixture"
)

func TestGeneratedScaffolds(t *testing.T) {
	tomlText, err := configbind.ScaffoldTOML()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(tomlText, "[webserver]\n") {
		t.Fatalf("TOML scaffold=%q", tomlText)
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
