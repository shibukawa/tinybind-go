package fixture_test

import (
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
