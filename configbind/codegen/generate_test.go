package codegen

import (
	"strings"
	"testing"
)

func TestGenerateEmitsScaffoldFragmentRegistration(t *testing.T) {
	src, err := Generate("fixture", []Spec{{
		PackagePath: "example.test/fixture",
		TypeName:    "ServerConfig",
		Prefix:      "server",
		Fields:      []Field{{GoName: "Port", Key: "port", Kind: FieldInt, Default: "8080"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`configbind.RegisterBinding[ServerConfig]("server", "example.test/fixture.ServerConfig"`,
		`configbind.RegisterScaffold(configbind.ScaffoldFragment{ID: "example.test/fixture.ServerConfig@server"`,
		`{Key: "port", Kind: configbind.ScaffoldInt, Default: "8080"}`,
	} {
		if !strings.Contains(string(src), want) {
			t.Fatalf("generated scaffold registration %q missing:\n%s", want, src)
		}
	}
}

func TestGenerateSupportsSameTypeAtMultiplePrefixes(t *testing.T) {
	src, err := Generate("fixture", []Spec{
		{PackagePath: "example.test/fixture", TypeName: "ServerConfig", Prefix: "primary", Fields: []Field{{GoName: "Port", Key: "port", Kind: FieldInt}}},
		{PackagePath: "example.test/fixture", TypeName: "ServerConfig", Prefix: "secondary", Fields: []Field{{GoName: "Port", Key: "port", Kind: FieldInt}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"registerServerConfigBinding0",
		"registerServerConfigBinding1",
		"applyServerConfigBinding0",
		"applyServerConfigBinding1",
		`RegisterBinding[ServerConfig]("primary"`,
		`RegisterBinding[ServerConfig]("secondary"`,
	} {
		if !strings.Contains(string(src), want) {
			t.Fatalf("generated multi-prefix symbol %q missing:\n%s", want, src)
		}
	}
}

func TestGenerateEmitsEnvironmentOverride(t *testing.T) {
	src, err := Generate("fixture", []Spec{{
		TypeName: "ObservabilityConfig",
		Prefix:   "observability",
		Fields: []Field{
			{GoName: "ServiceName", Key: "service_name", Kind: FieldString, Env: "OTEL_SERVICE_NAME"},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), `Env: "OTEL_SERVICE_NAME"`) {
		t.Fatalf("generated environment override missing:\n%s", src)
	}
}

func TestGenerateRejectsDuplicateEnvironmentOverride(t *testing.T) {
	_, err := Generate("fixture", []Spec{{
		TypeName: "ObservabilityConfig",
		Prefix:   "observability",
		Fields: []Field{
			{GoName: "ServiceName", Key: "service_name", Kind: FieldString, Env: "OTEL_SERVICE_NAME"},
			{GoName: "PeerName", Key: "peer_name", Kind: FieldString, Env: "OTEL_SERVICE_NAME"},
		},
	}})
	if err == nil || !strings.Contains(err.Error(), "duplicate environment variable") {
		t.Fatalf("error=%v", err)
	}
}
