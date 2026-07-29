package fixture_test

import (
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/minitoml"
	"github.com/shibukawa/tinybind-go/minitoml/codegen/fixture"
)

// Exercises the committed generated Apply path in its own package (import boundary).
func TestApplyWebServiceConfigShipped(t *testing.T) {
	doc, err := minitoml.ParseString(`
[webservice]
listen_addr = ":7070"
cors_origins = ["x", "y"]
[webservice.tls]
enabled = true
cert_path = "c.pem"
`)
	if err != nil {
		t.Fatal(err)
	}
	var cfg fixture.WebServiceConfig
	if err := fixture.ApplyWebServiceConfig(&cfg, doc); err != nil {
		t.Fatal(err)
	}
	if cfg.ListenAddr != ":7070" || !cfg.TLS.Enabled || cfg.TLS.CertPath != "c.pem" {
		t.Fatalf("cfg=%+v", cfg)
	}
	if len(cfg.CorsOrigins) != 2 {
		t.Fatalf("CorsOrigins=%v", cfg.CorsOrigins)
	}
}

func TestApplyWebServiceConfigTableArray(t *testing.T) {
	doc, err := minitoml.ParseString(`
[webservice]
listen_addr = ":7070"

[[webservice.listeners]]
addr = ":8080"

[[webservice.listeners]]
addr = ":8443"
port = 8443
tls.enabled = true

[webservice.listeners.tls]
cert_path = "c.pem"
`)
	if err != nil {
		t.Fatal(err)
	}
	var cfg fixture.WebServiceConfig
	if err := fixture.ApplyWebServiceConfig(&cfg, doc); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Listeners) != 2 {
		t.Fatalf("Listeners=%+v", cfg.Listeners)
	}
	// Defaults apply per element, not once for the whole slice.
	if cfg.Listeners[0].Addr != ":8080" || cfg.Listeners[0].Port != 80 || cfg.Listeners[0].TLS.Enabled {
		t.Fatalf("Listeners[0]=%+v", cfg.Listeners[0])
	}
	if cfg.Listeners[1].Port != 8443 || !cfg.Listeners[1].TLS.Enabled || cfg.Listeners[1].TLS.CertPath != "c.pem" {
		t.Fatalf("Listeners[1]=%+v", cfg.Listeners[1])
	}
}

func TestApplyWebServiceConfigRejectsScalarForTableArray(t *testing.T) {
	doc, err := minitoml.ParseString("[webservice]\nlisteners = 3\n")
	if err != nil {
		t.Fatal(err)
	}
	var cfg fixture.WebServiceConfig
	err = fixture.ApplyWebServiceConfig(&cfg, doc)
	if err == nil {
		t.Fatal("expected an error for a scalar in place of an array of tables")
	}
	if !strings.Contains(err.Error(), "webservice.listeners") {
		t.Fatalf("error %q must name the key", err)
	}
}
