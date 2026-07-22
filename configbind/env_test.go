package configbind_test

import (
	"testing"

	"github.com/shibukawa/tinybind-go/cliparser"
	"github.com/shibukawa/tinybind-go/configbind"
)

func TestEnvNameFromLongOpt(t *testing.T) {
	if got := configbind.EnvName("port"); got != "PORT" {
		t.Fatalf("port -> %q", got)
	}
	if got := configbind.EnvName("webserver-host"); got != "WEBSERVER_HOST" {
		t.Fatalf("webserver-host -> %q", got)
	}
	if got := configbind.EnvName("webserver-tls-cert_path"); got != "WEBSERVER_TLS_CERT_PATH" {
		t.Fatalf("nested long -> %q", got)
	}
	if got := configbind.EnvName(cliparser.DefaultLongName("middleware.cache", "max_entries")); got != "MIDDLEWARE_CACHE_MAX_ENTRIES" {
		t.Fatalf("dotted prefix -> %q", got)
	}
}

func TestReadEnvUsesLongOptNames(t *testing.T) {
	defs := []cliparser.Def{
		{ConfigKey: "webserver.port", Longs: []string{"port"}},
		{ConfigKey: "webserver.host", Longs: []string{"webserver-host"}},
	}
	environ := []string{
		"PORT=8080",
		"WEBSERVER_PORT=9999", // must not map when long is "port"
		"OTHER=1",
	}
	m := configbind.ReadEnv(defs, environ)
	if m["webserver.port"] != "8080" {
		t.Fatalf("map=%v want PORT for webserver.port", m)
	}
	if _, ok := m["webserver.host"]; ok {
		t.Fatalf("unset host should be absent: %v", m)
	}
}

func TestReadEnvUsesExplicitOverrideAndDisable(t *testing.T) {
	defs, err := cliparser.BuildDefs([]cliparser.FieldMeta{
		{Prefix: "observability", Key: "service_name", Env: "OTEL_SERVICE_NAME"},
		{Prefix: "observability", Key: "endpoint", Env: "-"},
	})
	if err != nil {
		t.Fatal(err)
	}
	m := configbind.ReadEnv(defs, []string{
		"OTEL_SERVICE_NAME=checkout",
		"OBSERVABILITY_SERVICE_NAME=ignored",
		"OBSERVABILITY_ENDPOINT=https://ignored.example.com",
	})
	if got := m["observability.service_name"]; got != "checkout" {
		t.Fatalf("service name=%q", got)
	}
	if _, ok := m["observability.endpoint"]; ok {
		t.Fatalf("disabled environment field was loaded: %v", m)
	}
}
