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

func TestGenerateIntegerFieldWidths(t *testing.T) {
	src, err := codegen.Generate("fixture", []codegen.Spec{{
		TypeName: "LimitConfig",
		Prefix:   "limit",
		Fields: []codegen.Field{
			{GoName: "MaxBody", Key: "max_body", Kind: codegen.FieldInt, GoType: "int64"},
			{GoName: "Workers", Key: "workers", Kind: codegen.FieldInt, GoType: "uint16", Default: "4"},
			{GoName: "Port", Key: "port", Kind: codegen.FieldInt},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	for _, want := range []string{
		// int64 is what AsInt already returns, so it needs no range guard.
		"dst.MaxBody = int64(n)",
		"if n < 0 {",
		"if int64(uint16(n)) != n {",
		"dst.Workers = uint16(n)",
		// An empty GoType stays int, and the guard catches a 32-bit target.
		"if int64(int(n)) != n {",
		"dst.Port = int(n)",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in\n%s", want, text)
		}
	}
	if strings.Contains(text, "if int64(int64(n)) != n {") {
		t.Fatalf("int64 needs no range guard:\n%s", text)
	}
}

func TestGenerateRejectsOutOfRangeIntDefault(t *testing.T) {
	_, err := codegen.Generate("fixture", []codegen.Spec{{
		TypeName: "LimitConfig",
		Prefix:   "limit",
		Fields:   []codegen.Field{{GoName: "Workers", Key: "workers", Kind: codegen.FieldInt, GoType: "uint8", Default: "300"}},
	}})
	if err == nil || !strings.Contains(err.Error(), "invalid uint8 default") {
		t.Fatalf("err=%v want an out-of-range default rejection", err)
	}
}

// A float field reads a TOML float and widens a TOML integer; the narrower
// width converts, and a default is rendered as a float literal.
func TestGenerateFloatFields(t *testing.T) {
	var b bytes.Buffer
	spec := codegen.Spec{
		TypeName: "Gauge",
		Fields: []codegen.Field{
			{GoName: "Ratio", Key: "ratio", Kind: codegen.FieldFloat, Default: "0.5"},
			{GoName: "Scale", Key: "scale", Kind: codegen.FieldFloat, GoType: "float32"},
		},
	}
	err := codegen.EmitApply(&b, spec, codegen.ApplyOptions{FuncName: "fill", ErrorPrefix: "gauge", AllowEmptyPrefix: true})
	if err != nil {
		t.Fatalf("EmitApply: %v", err)
	}
	src := b.String()
	for _, want := range []string{
		"func fill(dst *Gauge, doc minitoml.Document) error",
		`doc.Get("ratio")`,
		"x, err := v.AsFloat()",
		"dst.Ratio = x",
		"dst.Ratio = 0.5",
		"dst.Scale = float32(x)",
		`"gauge: scale: %w"`,
		`"gauge: nil destination"`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("emitted source lacks %q\n%s", want, src)
		}
	}
	if strings.Contains(src, "minitoml: ") {
		t.Errorf("the error prefix was not replaced everywhere\n%s", src)
	}
	if err := codegen.EmitApply(&b, codegen.Spec{TypeName: "G", Fields: []codegen.Field{{GoName: "R", Key: "r", Kind: codegen.FieldFloat, Default: "half"}}}, codegen.ApplyOptions{AllowEmptyPrefix: true}); err == nil {
		t.Error("an unparsable float default was accepted")
	}
	if err := codegen.EmitApply(&b, codegen.Spec{TypeName: "G", Fields: []codegen.Field{{GoName: "R", Key: "r", Kind: codegen.FieldFloat, GoType: "complex64"}}}, codegen.ApplyOptions{AllowEmptyPrefix: true}); err == nil {
		t.Error("a non-float GoType was accepted")
	}
}

// AsFloat is what the emitted float field reads through.
func TestValueAsFloat(t *testing.T) {
	doc, err := minitoml.ParseString("a = 1.5\nb = 2\nc = \"x\"\n")
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]float64{"a": 1.5, "b": 2} {
		v, _ := doc.Get(key)
		got, err := v.AsFloat()
		if err != nil || got != want {
			t.Errorf("%s: got %v %v", key, got, err)
		}
	}
	v, _ := doc.Get("c")
	if _, err := v.AsFloat(); err == nil {
		t.Error("a non-numeric string converted to float")
	}
}

// The zero options emit exactly the form Generate does, so the pinned
// fixture and a caller of EmitApply cannot drift.
func TestEmitApplyZeroOptionsMatchesGenerate(t *testing.T) {
	var b bytes.Buffer
	if err := codegen.EmitApply(&b, codegen.WebServiceFixtureSpec(), codegen.ApplyOptions{}); err != nil {
		t.Fatal(err)
	}
	whole, err := codegen.Generate("fixture", []codegen.Spec{codegen.WebServiceFixtureSpec()})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(whole), strings.TrimSpace(b.String())) {
		t.Fatalf("EmitApply output is not what Generate writes\n%s", b.String())
	}
	if _, err := codegen.Generate("fixture", []codegen.Spec{{TypeName: "NoPrefix"}}); err == nil {
		t.Error("Generate accepted an empty prefix")
	}
}
