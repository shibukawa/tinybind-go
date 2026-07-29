package configbindfixture_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/configbind"
	"github.com/shibukawa/tinybind-go/internal/configbindfixture"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func loadWith(t *testing.T, body string, environ []string) (*configbindfixture.WebServerConfig, *configbind.LoadResult, error) {
	t.Helper()
	configbind.ResetTargets()
	cfg := configbindfixture.Register()
	res, err := configbind.Load(configbind.LoadOptions{
		Vendor:  "acme",
		Tool:    "demo",
		Environ: environ,
		Args:    []string{"--config-path", writeConfig(t, body)},
	})
	return cfg, res, err
}

// The motivating case: each [[webserver.routes]] element carries its own reference,
// so repeated settings take outside input even though an element has no flag and
// no environment variable of its own.
func TestInterpolationReachesTableArrayElements(t *testing.T) {
	body := `
[webserver]
host = "${PUBLIC_HOST}"

[[webserver.routes]]
path = "/"
dir = "${ASSET_ROOT}/public"

[[webserver.routes]]
path = "/files"
dir = "${ASSET_ROOT}/files"
`
	cfg, res, err := loadWith(t, body, []string{
		"PUBLIC_HOST=www.example",
		"ASSET_ROOT=/srv/app",
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Host != "www.example" {
		t.Fatalf("Host=%q", cfg.Host)
	}
	if len(cfg.Routes) != 2 {
		t.Fatalf("Routes=%v", cfg.Routes)
	}
	if cfg.Routes[0].Dir != "/srv/app/public" {
		t.Fatalf("Routes[0].Dir=%q", cfg.Routes[0].Dir)
	}
	if cfg.Routes[1].Dir != "/srv/app/files" {
		t.Fatalf("Routes[1].Dir=%q", cfg.Routes[1].Dir)
	}
	// An expanded value is still the file's, so later layers keep overriding it.
	if e, ok := res.Overlay.Get("webserver.host"); !ok || e.Place != configbind.PlaceFile {
		t.Fatalf("host place=%v ok=%v want file_toml", e.Place, ok)
	}
}

func TestInterpolationInArrayElements(t *testing.T) {
	body := `
[webserver]
cors_origins = ["https://${APEX}", "https://admin.${APEX}", "https://fixed.example"]
`
	cfg, _, err := loadWith(t, body, []string{"APEX=example.com"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := []string{"https://example.com", "https://admin.example.com", "https://fixed.example"}
	if len(cfg.CorsOrigins) != len(want) {
		t.Fatalf("CorsOrigins=%v", cfg.CorsOrigins)
	}
	for i := range want {
		if cfg.CorsOrigins[i] != want[i] {
			t.Fatalf("CorsOrigins[%d]=%q want %q", i, cfg.CorsOrigins[i], want[i])
		}
	}
}

// A reference in a quoted value still reaches a typed field: the overlay is
// string-based and the generated apply parses afterwards.
func TestInterpolationIntoTypedFields(t *testing.T) {
	body := `
[webserver]
port = "${LISTEN_PORT}"
read_timeout = "${SLOW_TIMEOUT}"
`
	cfg, _, err := loadWith(t, body, []string{"LISTEN_PORT=9443", "SLOW_TIMEOUT=30s"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 9443 {
		t.Fatalf("Port=%d", cfg.Port)
	}
	if cfg.ReadTimeout.String() != "30s" {
		t.Fatalf("ReadTimeout=%s", cfg.ReadTimeout)
	}
}

// An undefined name fails the load. Silently expanding to "" would let a missing
// secret erase a default tag value, because the file layer outranks defaults.
func TestInterpolationUndefinedVariableFails(t *testing.T) {
	body := `
[webserver]
host = "${MISSING_HOST}"
`
	_, _, err := loadWith(t, body, []string{"OTHER=1"})
	if err == nil {
		t.Fatal("Load succeeded, want undefined variable error")
	}
	for _, want := range []string{"MISSING_HOST", "webserver.host", "undefined"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not mention %q", err, want)
		}
	}
}

// An error inside a repeated table names the element it came from; the bare key
// would be ambiguous across elements.
func TestInterpolationErrorNamesTableArrayElement(t *testing.T) {
	body := `
[[webserver.routes]]
path = "/"
dir = "/srv/ok"

[[webserver.routes]]
path = "/files"
dir = "${MISSING_ROOT}/files"
`
	_, _, err := loadWith(t, body, []string{})
	if err == nil {
		t.Fatal("Load succeeded, want undefined variable error")
	}
	if !strings.Contains(err.Error(), "webserver.routes[1].dir") {
		t.Fatalf("error %q does not name the failing element", err)
	}
}

// Expansion belongs to the file layer only, so a reference written in an env or
// CLI value stays literal and no expansion loop is possible.
func TestInterpolationDoesNotApplyToEnvOrCLIValues(t *testing.T) {
	configbind.ResetTargets()
	cfg := configbindfixture.Register()
	path := writeConfig(t, "[webserver]\nport = 1\n")
	res, err := configbind.Load(configbind.LoadOptions{
		Vendor:  "acme",
		Tool:    "demo",
		Environ: []string{"WEBSERVER_HOST=${NOT_EXPANDED}", "NOT_EXPANDED=surprise"},
		Args:    []string{"--config-path", path, "--webserver-tracing_url", "${ALSO_NOT}"},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Host != "${NOT_EXPANDED}" {
		t.Fatalf("Host=%q want the literal reference", cfg.Host)
	}
	if cfg.TracingURL != "${ALSO_NOT}" {
		t.Fatalf("TracingURL=%q want the literal reference", cfg.TracingURL)
	}
	if e, ok := res.Overlay.Get("webserver.host"); !ok || e.Place != configbind.PlaceEnv {
		t.Fatalf("host place=%v ok=%v want env", e.Place, ok)
	}
}

// $$ collapses to one $, which is what lets a literal dollar survive a layer
// that also reads ${NAME}.
func TestInterpolationEscapedDollarInFile(t *testing.T) {
	body := `
[webserver]
host = "pa$$word.example"
`
	cfg, _, err := loadWith(t, body, []string{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Host != "pa$word.example" {
		t.Fatalf("Host=%q", cfg.Host)
	}
}
