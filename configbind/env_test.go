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
