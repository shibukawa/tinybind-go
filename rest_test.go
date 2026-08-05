package httpbind_test

import (
	"testing"

	httpbind "github.com/shibukawa/tinybind-go"
	"github.com/shibukawa/tinybind-go/jsonbind"
)

func TestRestJSONAny_ExcludesAndDecodesNested(t *testing.T) {
	body, err := jsonbind.ParseObject([]byte(
		`{"name":"Ada","role":"admin","meta":{"source":"import"},"count":2}`))
	if err != nil {
		t.Fatal(err)
	}
	got, err := httpbind.RestJSONAny(body, []string{"name", "email"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got["name"]; ok {
		t.Fatalf("name excluded: %#v", got)
	}
	if got["role"] != "admin" {
		t.Fatalf("role: %#v", got["role"])
	}
	meta, ok := got["meta"].(map[string]any)
	if !ok || meta["source"] != "import" {
		t.Fatalf("meta: %#v", got["meta"])
	}
	// JSON numbers decode as float64 into any
	if got["count"] != float64(2) {
		t.Fatalf("count: %#v (%T)", got["count"], got["count"])
	}
}

func TestRestJSONNames_ExcludesDeclared(t *testing.T) {
	body, err := jsonbind.ParseObject([]byte(`{"name":"x","k":{"a":1}}`))
	if err != nil {
		t.Fatal(err)
	}
	got := httpbind.RestJSONNames(body, []string{"name"})
	if len(got) != 1 || got[0] != "k" {
		t.Fatalf("rest names: %#v", got)
	}
	raw, ok := body.Get("k")
	if !ok || string(raw) != `{"a":1}` {
		t.Fatalf("raw value: %q ok=%v", raw, ok)
	}
}

func TestRestFormAny(t *testing.T) {
	got := httpbind.RestFormAny(map[string]string{
		"name": "n",
		"x":    "1",
	}, []string{"name"})
	if got["x"] != "1" {
		t.Fatalf("%#v", got)
	}
	if _, ok := got["name"]; ok {
		t.Fatalf("name in rest: %#v", got)
	}
}
