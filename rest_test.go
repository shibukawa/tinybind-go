package httpbind_test

import (
	"testing"

	httpbind "github.com/shibukawa/tinybind-go"
)

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
