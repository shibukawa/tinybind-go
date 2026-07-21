package minitoml_test

import (
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/minitoml"
)

func TestParseAllowedNestedDottedAndArray(t *testing.T) {
	src := `
# sample webservice config
[webservice]
listen_addr = ":8080"
cors_origins = ["https://a.example", "https://b.example"]
tls.enabled = true
max_conns = 42

[webservice.tls]
cert_path = "server.crt"
`
	doc, err := minitoml.ParseString(src)
	if err != nil {
		t.Fatalf("ParseString: %v", err)
	}

	// Nested/dotted paths become stable hierarchical keys.
	wantKeys := []string{
		"webservice.cors_origins",
		"webservice.listen_addr",
		"webservice.max_conns",
		"webservice.tls.cert_path",
		"webservice.tls.enabled",
	}
	gotKeys := doc.Keys()
	if len(gotKeys) != len(wantKeys) {
		t.Fatalf("keys=%v want %v", gotKeys, wantKeys)
	}
	for i, k := range wantKeys {
		if gotKeys[i] != k {
			t.Fatalf("keys[%d]=%q want %q (all=%v)", i, gotKeys[i], k, gotKeys)
		}
	}

	v, ok := doc.Get("webservice.listen_addr")
	if !ok || v.Kind != minitoml.KindString || v.Str != ":8080" {
		t.Fatalf("listen_addr=%v ok=%v", v, ok)
	}

	// Primitive array preserved as multi-values, not a single string.
	arr, ok := doc.Get("webservice.cors_origins")
	if !ok || arr.Kind != minitoml.KindArray {
		t.Fatalf("cors_origins kind=%v ok=%v", arr.Kind, ok)
	}
	sl, err := arr.AsStringSlice()
	if err != nil {
		t.Fatalf("AsStringSlice: %v", err)
	}
	if len(sl) != 2 || sl[0] != "https://a.example" || sl[1] != "https://b.example" {
		t.Fatalf("cors_origins=%v", sl)
	}
	if arr.String() == `["https://a.example", "https://b.example"]` && len(arr.Array) != 2 {
		t.Fatalf("array should keep elements, got %#v", arr)
	}
	if len(arr.Array) != 2 {
		t.Fatalf("array elements not preserved: %#v", arr)
	}

	en, ok := doc.Get("webservice.tls.enabled")
	if !ok || en.Kind != minitoml.KindBool || !en.Bool {
		t.Fatalf("tls.enabled=%v ok=%v", en, ok)
	}
	cert, ok := doc.Get("webservice.tls.cert_path")
	if !ok || cert.Str != "server.crt" {
		t.Fatalf("tls.cert_path=%v ok=%v", cert, ok)
	}
	mc, ok := doc.Get("webservice.max_conns")
	if !ok || mc.Kind != minitoml.KindInt || mc.Int != 42 {
		t.Fatalf("max_conns=%v ok=%v", mc, ok)
	}
}

func TestParseForbiddenShapes(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		wantSub string
	}{
		{
			name:    "quoted_key",
			src:     `"listen-addr" = ":8080"` + "\n",
			wantSub: "quoted keys are not allowed",
		},
		{
			name:    "quoted_key_in_table",
			src:     "[webservice]\n\"listen-addr\" = \":8080\"\n",
			wantSub: "quoted keys are not allowed",
		},
		{
			name:    "inline_table",
			src:     "tls = { enabled = true }\n",
			wantSub: "inline tables are not allowed",
		},
		{
			name:    "array_of_tables",
			src:     "[[webservice.listeners]]\naddr = \":8080\"\n",
			wantSub: "arrays of tables are not allowed",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := minitoml.ParseString(tc.src)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantSub)
			}
		})
	}
}

func TestParseTopLevelAndLiteralString(t *testing.T) {
	doc, err := minitoml.ParseString("name = 'plain'\nenabled = false\n")
	if err != nil {
		t.Fatal(err)
	}
	v, ok := doc.Get("name")
	if !ok || v.Str != "plain" {
		t.Fatalf("name=%v", v)
	}
	b, ok := doc.Get("enabled")
	if !ok || b.Bool {
		t.Fatalf("enabled=%v", b)
	}
}
