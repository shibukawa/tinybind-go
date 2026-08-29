package configbindfixture_test

import (
	"strings"
	"testing"
)

// A []string field set from the environment is a comma-separated list: env
// carries only KEY=value, so this is its only spelling of a list. It used to
// arrive as one bogus element ("a.example,b.example"); it now splits.
func TestEnvListSplitsOnComma(t *testing.T) {
	cfg, _, err := loadWith(t, "", []string{"WEBSERVER_CORS_ORIGINS=a.example,b.example, c.example"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := []string{"a.example", "b.example", "c.example"}
	if strings.Join(cfg.CorsOrigins, "|") != strings.Join(want, "|") {
		t.Fatalf("CorsOrigins = %v, want %v", cfg.CorsOrigins, want)
	}
}

// The same split feeds the enum check, so a comma-separated env value is
// validated element by element rather than as one unsplit token that could
// never match.
func TestEnvEnumListSplitsBeforeValidation(t *testing.T) {
	cfg, _, err := loadWith(t, "", []string{"WEBSERVER_PROTOCOLS=http1,http2"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if strings.Join(cfg.Protocols, ",") != "http1,http2" {
		t.Fatalf("Protocols = %v, want [http1 http2]", cfg.Protocols)
	}

	if _, _, err := loadWith(t, "", []string{"WEBSERVER_PROTOCOLS=http1,nope"}); err == nil {
		t.Fatal("an out-of-set element in an env list was accepted")
	}
}
