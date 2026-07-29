package codegen_test

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/minitoml"
	"github.com/shibukawa/tinybind-go/minitoml/codegen"
	"github.com/shibukawa/tinybind-go/minitoml/codegen/fixture"
)

func TestGenerateEmitsIntermediateKeys(t *testing.T) {
	src, err := codegen.Generate("fixture", []codegen.Spec{codegen.WebServiceFixtureSpec()})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	text := string(src)
	// Generated apply must hard-code intermediate keys (no reflection).
	for _, key := range []string{
		`"webservice.listen_addr"`,
		`"webservice.max_conns"`,
		`"webservice.cors_origins"`,
		`"webservice.tls.enabled"`,
		`"webservice.tls.cert_path"`,
	} {
		if !strings.Contains(text, key) {
			t.Fatalf("generated source missing key %s\n%s", key, text)
		}
	}
	if strings.Contains(text, "reflect.") {
		t.Fatal("generated code must not use reflect")
	}
	if !strings.Contains(text, "func ApplyWebServiceConfig") {
		t.Fatal("missing ApplyWebServiceConfig")
	}
}

func TestGenerateMatchesCommittedFixture(t *testing.T) {
	src, err := codegen.Generate("fixture", []codegen.Spec{codegen.WebServiceFixtureSpec()})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	committedPath := filepath.Join(filepath.Dir(file), "fixture", "apply_gen.go")
	want, err := os.ReadFile(committedPath)
	if err != nil {
		t.Fatalf("read committed apply_gen.go: %v", err)
	}
	if !bytes.Equal(src, want) {
		t.Fatalf("committed fixture/apply_gen.go is out of date; re-run generator\n--- got ---\n%s\n--- want ---\n%s", src, want)
	}
}

func TestGeneratedApplyFromParsedTOML(t *testing.T) {
	// Real shipped path: minitoml.Parse → intermediate Document → generated Apply*.
	const tomlSrc = `
[webservice]
listen_addr = ":9090"
cors_origins = ["https://a.example", "https://b.example"]
max_conns = 7
tls.enabled = true

[webservice.tls]
cert_path = "server.crt"
`
	doc, err := minitoml.ParseString(tomlSrc)
	if err != nil {
		t.Fatalf("ParseString: %v", err)
	}

	// Intermediate keys present before apply.
	if _, ok := doc.Get("webservice.cors_origins"); !ok {
		t.Fatal("missing intermediate key webservice.cors_origins")
	}
	if _, ok := doc.Get("webservice.tls.cert_path"); !ok {
		t.Fatal("missing intermediate key webservice.tls.cert_path")
	}

	var cfg fixture.WebServiceConfig
	if err := fixture.ApplyWebServiceConfig(&cfg, doc); err != nil {
		t.Fatalf("ApplyWebServiceConfig: %v", err)
	}
	if cfg.ListenAddr != ":9090" {
		t.Fatalf("ListenAddr=%q", cfg.ListenAddr)
	}
	if cfg.MaxConns != 7 {
		t.Fatalf("MaxConns=%d", cfg.MaxConns)
	}
	if len(cfg.CorsOrigins) != 2 || cfg.CorsOrigins[0] != "https://a.example" || cfg.CorsOrigins[1] != "https://b.example" {
		t.Fatalf("CorsOrigins=%v", cfg.CorsOrigins)
	}
	if !cfg.TLS.Enabled || cfg.TLS.CertPath != "server.crt" {
		t.Fatalf("TLS=%+v", cfg.TLS)
	}
}

func TestGeneratedApplyUsesDefaultsWhenKeysAbsent(t *testing.T) {
	doc := minitoml.NewDocument()
	var cfg fixture.WebServiceConfig
	if err := fixture.ApplyWebServiceConfig(&cfg, doc); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if cfg.ListenAddr != ":8080" {
		t.Fatalf("default ListenAddr=%q", cfg.ListenAddr)
	}
	if cfg.MaxConns != 100 {
		t.Fatalf("default MaxConns=%d", cfg.MaxConns)
	}
	if cfg.TLS.Enabled {
		t.Fatalf("default TLS.Enabled should be false")
	}
}

func TestGenerateEmitsDurationField(t *testing.T) {
	src, err := codegen.Generate("fixture", []codegen.Spec{{
		TypeName: "ServerConfig",
		Prefix:   "server",
		Fields:   []codegen.Field{{GoName: "ReadTimeout", Key: "read_timeout", Kind: codegen.FieldDuration, Default: "250ms"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"time"`,
		"time.ParseDuration(s)",
		"dst.ReadTimeout = 250000000 // 250ms",
	} {
		if !strings.Contains(string(src), want) {
			t.Fatalf("generated duration field %q missing:\n%s", want, src)
		}
	}
}

func TestGenerateRejectsUnparsableDurationDefault(t *testing.T) {
	_, err := codegen.Generate("fixture", []codegen.Spec{{
		TypeName: "ServerConfig",
		Prefix:   "server",
		Fields:   []codegen.Field{{GoName: "ReadTimeout", Key: "read_timeout", Kind: codegen.FieldDuration, Default: "250"}},
	}})
	if err == nil || !strings.Contains(err.Error(), "invalid duration default") {
		t.Fatalf("err=%v want a rejected bare-number default", err)
	}
}
func TestGenerateStructSliceRequiresElemType(t *testing.T) {
	spec := codegen.Spec{
		TypeName: "Config",
		Prefix:   "app",
		Fields: []codegen.Field{{
			GoName: "Listeners",
			Key:    "listeners",
			Kind:   codegen.FieldStructSlice,
			Nested: []codegen.Field{{GoName: "Addr", Key: "addr", Kind: codegen.FieldString}},
		}},
	}
	if _, err := codegen.Generate("fixture", []codegen.Spec{spec}); err == nil {
		t.Fatal("expected an error when ElemType is missing")
	}
}

func TestGenerateNestedStructSlicesUseDistinctLoopVariables(t *testing.T) {
	spec := codegen.Spec{
		TypeName: "Config",
		Prefix:   "app",
		Fields: []codegen.Field{{
			GoName:   "Servers",
			Key:      "servers",
			Kind:     codegen.FieldStructSlice,
			ElemType: "Server",
			Nested: []codegen.Field{{
				GoName:   "Disks",
				Key:      "disks",
				Kind:     codegen.FieldStructSlice,
				ElemType: "Disk",
				Nested:   []codegen.Field{{GoName: "Mount", Key: "mount", Kind: codegen.FieldString}},
			}},
		}},
	}
	// format.Source would already reject a redeclared loop variable; assert the
	// nesting is emitted the way the element documents are shaped.
	src, err := codegen.Generate("fixture", []codegen.Spec{spec})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	text := string(src)
	for _, want := range []string{
		`doc.Get("app.servers")`,
		`for i1 := range elems1 {`,
		`elems1[i1].Get("disks")`,
		`for i2 := range elems2 {`,
		`dst.Servers[i1].Disks[i2].Mount = s`,
		`"minitoml: app.servers.disks.mount: %w"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("generated source missing %s\n%s", want, text)
		}
	}
}
