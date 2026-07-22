package codegen_test

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/cliparser"
	"github.com/shibukawa/tinybind-go/cliparser/codegen"
	"github.com/shibukawa/tinybind-go/cliparser/codegen/fixture"
)

func TestGenerateEmitsStableKeysAndOptFlags(t *testing.T) {
	src, err := codegen.Generate("fixture", "WebServerFlagDefs", codegen.WebServerFixtureFields())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	text := string(src)
	for _, want := range []string{
		`"webserver.port"`,
		`[]string{"port"}`,
		`[]string{"p"}`,
		`"webserver.read_timeout"`,
		`[]string{"webserver-read_timeout"}`,
		`"webserver.tls.enabled"`,
		`[]string{"webserver-tls-enabled"}`,
		`UsesOptOverride: true`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("generated missing %q\n%s", want, text)
		}
	}
	// Default prefixed long for port must not appear when opt is set.
	if strings.Contains(text, `"webserver-port"`) {
		t.Fatalf("opt field must not emit default long webserver-port\n%s", text)
	}
	if strings.Contains(text, "reflect.") {
		t.Fatal("generated code must not use reflect")
	}
}

func TestGenerateEmitsEnvironmentOverride(t *testing.T) {
	src, err := codegen.Generate("fixture", "ObservabilityFlagDefs", []cliparser.FieldMeta{
		{Prefix: "observability", Key: "service_name", Env: "OTEL_SERVICE_NAME"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), `Env:       "OTEL_SERVICE_NAME"`) {
		t.Fatalf("generated environment override missing:\n%s", src)
	}
}

func TestGenerateMatchesCommittedFixture(t *testing.T) {
	src, err := codegen.Generate("fixture", "WebServerFlagDefs", codegen.WebServerFixtureFields())
	if err != nil {
		t.Fatal(err)
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	path := filepath.Join(filepath.Dir(file), "fixture", "flagdefs_gen.go")
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read committed: %v", err)
	}
	if !bytes.Equal(src, want) {
		t.Fatalf("fixture/flagdefs_gen.go out of date\n--- got ---\n%s\n--- want ---\n%s", src, want)
	}
}

func TestGeneratedDefsParseArgv(t *testing.T) {
	// Real path: codegen.Generate → committed fixture.WebServerFlagDefs → cliparser.Parse.
	defs := fixture.WebServerFlagDefs()
	if len(defs) == 0 {
		t.Fatal("empty defs")
	}

	// Build expected defs via shipped helper and ensure fixture matches naming rules.
	wantDefs, err := cliparser.BuildDefs(codegen.WebServerFixtureFields())
	if err != nil {
		t.Fatal(err)
	}
	if len(defs) != len(wantDefs) {
		t.Fatalf("len defs=%d want %d", len(defs), len(wantDefs))
	}
	for i := range defs {
		if defs[i].ConfigKey != wantDefs[i].ConfigKey {
			t.Fatalf("defs[%d].ConfigKey=%q want %q", i, defs[i].ConfigKey, wantDefs[i].ConfigKey)
		}
	}

	res, err := cliparser.Parse([]string{
		"--port", "8080",
		"--webserver-read_timeout", "3s",
		"--webserver-tls-enabled",
		"--webserver-cors_origins", "a",
		"--webserver-cors_origins", "b",
	}, defs)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if res.Values["webserver.port"] != "8080" {
		t.Fatalf("port=%v", res.Values)
	}
	if res.Values["webserver.read_timeout"] != "3s" {
		t.Fatalf("timeout=%v", res.Values)
	}
	if res.Values["webserver.tls.enabled"] != "true" {
		t.Fatalf("tls=%v", res.Values)
	}
	if got := res.Multi["webserver.cors_origins"]; len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("cors=%v", res.Multi)
	}

	// Suppressed default long for port.
	if _, err := cliparser.Parse([]string{"--webserver-port", "1"}, defs); err == nil {
		t.Fatal("expected unknown --webserver-port")
	}
}
