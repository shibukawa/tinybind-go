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
			name:    "array_of_inline_tables",
			src:     "listeners = [{ addr = \":8080\" }]\n",
			wantSub: "inline tables are not allowed",
		},
		{
			name:    "table_array_over_scalar",
			src:     "[webservice]\nlisteners = 1\n[[webservice.listeners]]\naddr = \":8080\"\n",
			wantSub: "conflicts with key",
		},
		{
			name:    "scalar_over_table_array",
			src:     "[[webservice.listeners]]\naddr = \":8080\"\n[webservice]\nlisteners = 1\n",
			wantSub: "is an array of tables",
		},
		{
			name:    "table_reopens_table_array",
			src:     "[[listeners]]\naddr = \":8080\"\n[other]\nx = 1\n[listeners]\naddr = \":9090\"\n",
			wantSub: "use a [[listeners]] header",
		},
		{
			name:    "subtable_outside_element",
			src:     "[[listeners]]\naddr = \":8080\"\n[other]\nx = 1\n[listeners.tls]\nenabled = true\n",
			wantSub: "must follow its [[listeners]] header",
		},
		{
			name:    "unterminated_table_array_header",
			src:     "[[listeners]\naddr = \":8080\"\n",
			wantSub: "expected ']]'",
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

func TestParseTableArray(t *testing.T) {
	src := `
[webservice]
listen_addr = ":8080"

[[webservice.listeners]]
addr = ":8080"
tls.enabled = false

[[webservice.listeners]]
addr = ":8443"

# a standard table under the open element is that element's sub-table
[webservice.listeners.tls]
enabled = true
cert_path = "server.crt"

[[webservice.listeners]]
addr = ":9090"

[webservice.limits]
max_conns = 42
`
	doc, err := minitoml.ParseString(src)
	if err != nil {
		t.Fatalf("ParseString: %v", err)
	}

	// The array occupies one key; its elements are nested, not flattened.
	wantKeys := []string{
		"webservice.limits.max_conns",
		"webservice.listen_addr",
		"webservice.listeners",
	}
	if got := doc.Keys(); !equalStrings(got, wantKeys) {
		t.Fatalf("keys=%v want %v", got, wantKeys)
	}

	v, ok := doc.Get("webservice.listeners")
	if !ok || v.Kind != minitoml.KindTableArray {
		t.Fatalf("listeners kind=%v ok=%v", v.Kind, ok)
	}
	tables, err := v.AsTables()
	if err != nil {
		t.Fatalf("AsTables: %v", err)
	}
	if len(tables) != 3 {
		t.Fatalf("len(tables)=%d want 3", len(tables))
	}

	wantAddrs := []string{":8080", ":8443", ":9090"}
	for i, want := range wantAddrs {
		// Element keys are relative to the array key itself.
		addr, ok := tables[i].Get("addr")
		if !ok || addr.Str != want {
			t.Fatalf("tables[%d].addr=%v ok=%v want %q", i, addr, ok, want)
		}
	}

	// Dotted keys and [table] headers both nest inside the element they follow.
	first, ok := tables[0].Get("tls.enabled")
	if !ok || first.Kind != minitoml.KindBool || first.Bool {
		t.Fatalf("tables[0].tls.enabled=%v ok=%v", first, ok)
	}
	second, ok := tables[1].Get("tls.enabled")
	if !ok || !second.Bool {
		t.Fatalf("tables[1].tls.enabled=%v ok=%v", second, ok)
	}
	cert, ok := tables[1].Get("tls.cert_path")
	if !ok || cert.Str != "server.crt" {
		t.Fatalf("tables[1].tls.cert_path=%v ok=%v", cert, ok)
	}
	if _, ok := tables[2].Get("tls.enabled"); ok {
		t.Fatal("tables[2] must not inherit the previous element's sub-table")
	}

	// A following standard table leaves the array element behind.
	mc, ok := doc.Get("webservice.limits.max_conns")
	if !ok || mc.Int != 42 {
		t.Fatalf("limits.max_conns=%v ok=%v", mc, ok)
	}
}

func TestParseNestedTableArray(t *testing.T) {
	doc, err := minitoml.ParseString(`
[[servers]]
name = "a"

[[servers.disks]]
mount = "/"

[[servers.disks]]
mount = "/var"

[[servers]]
name = "b"
`)
	if err != nil {
		t.Fatalf("ParseString: %v", err)
	}
	servers, ok := doc.Get("servers")
	if !ok {
		t.Fatal("servers missing")
	}
	tables, err := servers.AsTables()
	if err != nil {
		t.Fatalf("AsTables: %v", err)
	}
	if len(tables) != 2 {
		t.Fatalf("len(servers)=%d want 2", len(tables))
	}
	disks, ok := tables[0].Get("disks")
	if !ok {
		t.Fatal("servers[0].disks missing")
	}
	diskTables, err := disks.AsTables()
	if err != nil {
		t.Fatalf("AsTables: %v", err)
	}
	if len(diskTables) != 2 {
		t.Fatalf("len(disks)=%d want 2", len(diskTables))
	}
	if m, _ := diskTables[1].Get("mount"); m.Str != "/var" {
		t.Fatalf("disks[1].mount=%v", m)
	}
	// The second [[servers]] element starts empty.
	if _, ok := tables[1].Get("disks"); ok {
		t.Fatal("servers[1] must not inherit servers[0].disks")
	}
	if name, _ := tables[1].Get("name"); name.Str != "b" {
		t.Fatalf("servers[1].name=%v", name)
	}
}

func TestTableArrayValueAccessors(t *testing.T) {
	doc, err := minitoml.ParseString("[[listeners]]\naddr = \":8080\"\n")
	if err != nil {
		t.Fatal(err)
	}
	v, _ := doc.Get("listeners")
	if v.KindName() != "table array" {
		t.Fatalf("KindName=%q", v.KindName())
	}
	if got, want := v.String(), `[{addr = :8080}]`; got != want {
		t.Fatalf("String=%q want %q", got, want)
	}
	if _, err := v.AsString(); err == nil {
		t.Fatal("AsString on a table array must fail")
	}
	if _, err := v.AsStringSlice(); err == nil {
		t.Fatal("AsStringSlice on a table array must fail")
	}
	scalar, ok := v.Tables[0].Get("addr")
	if !ok {
		t.Fatal("element addr missing")
	}
	if _, err := scalar.AsTables(); err == nil {
		t.Fatal("AsTables on a scalar must fail")
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
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
