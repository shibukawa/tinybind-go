package configbindfixture_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shibukawa/tinybind-go/configbind"
	"github.com/shibukawa/tinybind-go/internal/configbindfixture"
)

func TestGeneratedDurationField(t *testing.T) {
	configbind.ResetTargets()
	cfg := configbindfixture.Register()

	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "config.toml")
	body := "[webserver]\nread_timeout = \"1h30m\"\n"
	if err := os.WriteFile(tomlPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := configbind.Load(configbind.LoadOptions{
		Vendor: "acme", Tool: "demo", Environ: []string{}, Args: []string{"--config-path", tomlPath},
	}); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ReadTimeout != 90*time.Minute {
		t.Fatalf("ReadTimeout=%v want 1h30m", cfg.ReadTimeout)
	}
}

func TestGeneratedDurationDefaultAndSources(t *testing.T) {
	for _, tc := range []struct {
		name    string
		environ []string
		args    []string
		want    time.Duration
	}{
		{name: "default", environ: []string{}, args: []string{}, want: 5 * time.Second},
		{name: "env", environ: []string{"WEBSERVER_READ_TIMEOUT=250ms"}, args: []string{}, want: 250 * time.Millisecond},
		{name: "cli", environ: []string{}, args: []string{"--webserver-read_timeout", "2m"}, want: 2 * time.Minute},
	} {
		t.Run(tc.name, func(t *testing.T) {
			configbind.ResetTargets()
			cfg := configbindfixture.Register()
			if _, err := configbind.Load(configbind.LoadOptions{
				Vendor: "acme", Tool: "demo", Environ: tc.environ, Args: tc.args,
			}); err != nil {
				t.Fatalf("Load: %v", err)
			}
			if cfg.ReadTimeout != tc.want {
				t.Fatalf("ReadTimeout=%v want %v", cfg.ReadTimeout, tc.want)
			}
		})
	}
}

func TestGeneratedDurationRejectsBareNumber(t *testing.T) {
	configbind.ResetTargets()
	configbindfixture.Register()
	_, err := configbind.Load(configbind.LoadOptions{
		Vendor: "acme", Tool: "demo", Args: []string{}, Environ: []string{"WEBSERVER_READ_TIMEOUT=5"},
	})
	if err == nil || !strings.Contains(err.Error(), "webserver.read_timeout") {
		t.Fatalf("err=%v want a read_timeout parse error", err)
	}
}

func TestProvenanceOrderFollowsDeclaration(t *testing.T) {
	configbind.ResetTargets()
	configbindfixture.Register()
	// Only keys present in the overlay are reported, so cert_path needs a value;
	// cors_origins has no default and stays absent. tracing appears anyway
	// because its falsy choice fills it in, and that in turn hides tracing_url.
	res, err := configbind.Load(configbind.LoadOptions{
		Vendor: "acme", Tool: "demo", Args: []string{},
		Environ: []string{"WEBSERVER_TLS_ENABLED=true", "TLS_CERT_FILE=x.crt"},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	var keys []string
	for _, entry := range res.Provenance() {
		keys = append(keys, entry.Key)
	}
	want := []string{
		"webserver.port",
		"webserver.host",
		"webserver.read_timeout",
		"webserver.tracing",
		"webserver.tls.enabled",
		"webserver.tls.cert_path",
	}
	if strings.Join(keys, ",") != strings.Join(want, ",") {
		t.Fatalf("keys=%v want %v", keys, want)
	}
}

func TestProvenanceHidesFieldsUnderEmptyParent(t *testing.T) {
	configbind.ResetTargets()
	cfg := configbindfixture.Register()
	// tls.enabled keeps its false default, so tls.cert_path is suppressed while
	// the parent itself stays visible.
	res, err := configbind.Load(configbind.LoadOptions{
		Vendor: "acme", Tool: "demo", Args: []string{}, Environ: []string{"TLS_CERT_FILE=hidden.crt"},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	seen := map[string]configbind.ProvenanceEntry{}
	for _, entry := range res.Provenance() {
		seen[entry.Key] = entry
	}
	if _, ok := seen["webserver.tls.cert_path"]; ok {
		t.Fatal("cert_path must be hidden while tls.enabled is false")
	}
	parent, ok := seen["webserver.tls.enabled"]
	if !ok {
		t.Fatal("the empty parent itself must stay visible")
	}
	if parent.Value != "false" || parent.Place != configbind.PlaceDefault {
		t.Fatalf("parent=%+v", parent)
	}
	// The struct field is still populated; only the log entry is suppressed.
	if cfg.TLS.CertPath != "hidden.crt" {
		t.Fatalf("CertPath=%q; dependon must not affect apply", cfg.TLS.CertPath)
	}
}

func TestProvenanceShowsDependentWhenParentSet(t *testing.T) {
	configbind.ResetTargets()
	configbindfixture.Register()
	res, err := configbind.Load(configbind.LoadOptions{
		Vendor: "acme", Tool: "demo", Args: []string{},
		Environ: []string{"WEBSERVER_TLS_ENABLED=true", "TLS_CERT_FILE=shown.crt"},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, entry := range res.Provenance() {
		if entry.Key == "webserver.tls.cert_path" {
			if entry.Value != "shown.crt" || entry.Place != configbind.PlaceEnv {
				t.Fatalf("cert_path=%+v", entry)
			}
			return
		}
	}
	t.Fatal("cert_path must appear once tls.enabled is true")
}

func TestFalsyFillsEmptyEnumValue(t *testing.T) {
	for _, tc := range []struct {
		name    string
		environ []string
		want    string
	}{
		{name: "unset", environ: []string{}, want: "off"},
		{name: "explicitly empty", environ: []string{"WEBSERVER_TRACING="}, want: "off"},
		{name: "chosen", environ: []string{"WEBSERVER_TRACING=otlp"}, want: "otlp"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			configbind.ResetTargets()
			cfg := configbindfixture.Register()
			if _, err := configbind.Load(configbind.LoadOptions{
				Vendor: "acme", Tool: "demo", Args: []string{}, Environ: tc.environ,
			}); err != nil {
				t.Fatalf("Load: %v", err)
			}
			if cfg.Tracing != tc.want {
				t.Fatalf("Tracing=%q want %q", cfg.Tracing, tc.want)
			}
		})
	}
}

func TestFalsyParentHidesItsDependents(t *testing.T) {
	for _, tc := range []struct {
		name       string
		environ    []string
		wantHidden bool
	}{
		{name: "falsy choice hides", environ: []string{"WEBSERVER_TRACING_URL=http://collector"}, wantHidden: true},
		{
			name:       "real choice shows",
			environ:    []string{"WEBSERVER_TRACING=otlp", "WEBSERVER_TRACING_URL=http://collector"},
			wantHidden: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			configbind.ResetTargets()
			cfg := configbindfixture.Register()
			res, err := configbind.Load(configbind.LoadOptions{
				Vendor: "acme", Tool: "demo", Args: []string{}, Environ: tc.environ,
			})
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			seen := map[string]configbind.ProvenanceEntry{}
			for _, entry := range res.Provenance() {
				seen[entry.Key] = entry
			}
			if _, ok := seen["webserver.tracing_url"]; ok == tc.wantHidden {
				t.Fatalf("tracing_url present=%v wantHidden=%v", ok, tc.wantHidden)
			}
			// The falsy parent itself is always reported, showing its own value.
			parent, ok := seen["webserver.tracing"]
			if !ok {
				t.Fatal("the falsy parent must stay visible")
			}
			if tc.wantHidden && parent.Value != "off" {
				t.Fatalf("parent=%+v want the falsy choice", parent)
			}
			// dependon never suppresses apply.
			if cfg.TracingURL != "http://collector" {
				t.Fatalf("TracingURL=%q; falsy must not affect apply", cfg.TracingURL)
			}
		})
	}
}

func TestDurationInsideTableArrayElement(t *testing.T) {
	configbind.ResetTargets()
	cfg := configbindfixture.Register()

	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "config.toml")
	body := `
[[webserver.routes]]
path = "/static"
dir = "./public"
max_age = "15m"

[[webserver.routes]]
path = "/assets"
dir = "./assets"
`
	if err := os.WriteFile(tomlPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := configbind.Load(configbind.LoadOptions{
		Vendor: "acme", Tool: "demo", Environ: []string{},
		Args: []string{"--config-path", tomlPath},
	}); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Routes) != 2 {
		t.Fatalf("Routes=%+v want 2 elements", cfg.Routes)
	}
	if cfg.Routes[0].MaxAge != 15*time.Minute {
		t.Fatalf("Routes[0].MaxAge=%v want 15m", cfg.Routes[0].MaxAge)
	}
	// The default applies once per element, not once per array.
	if cfg.Routes[1].MaxAge != time.Hour {
		t.Fatalf("Routes[1].MaxAge=%v want the 1h default", cfg.Routes[1].MaxAge)
	}
}

func TestDurationInsideTableArrayElementReportsFullKey(t *testing.T) {
	configbind.ResetTargets()
	configbindfixture.Register()

	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "config.toml")
	body := "[[webserver.routes]]\npath = \"/x\"\nmax_age = \"15\"\n"
	if err := os.WriteFile(tomlPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := configbind.Load(configbind.LoadOptions{
		Vendor: "acme", Tool: "demo", Environ: []string{},
		Args: []string{"--config-path", tomlPath},
	})
	if err == nil || !strings.Contains(err.Error(), "webserver.routes.max_age") {
		t.Fatalf("err=%v want the full element key in the message", err)
	}
}

// The admin token has a default, so it reaches the overlay and the generated
// struct; secret hide keeps it out of the log helper's output entirely.
func TestProvenanceDropsHiddenSecret(t *testing.T) {
	configbind.ResetTargets()
	cfg := configbindfixture.Register()
	res, err := configbind.Load(configbind.LoadOptions{
		Vendor: "acme", Tool: "demo", Args: []string{}, Environ: []string{},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AdminToken != "seed-token" {
		t.Fatalf("AdminToken=%q; hide must not change the bound value", cfg.AdminToken)
	}
	for _, entry := range res.Provenance() {
		if entry.Key == "webserver.admin_token" {
			t.Fatalf("hidden secret leaked into provenance: %+v", entry)
		}
	}
}
