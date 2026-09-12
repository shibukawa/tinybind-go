package httpbind_test

import (
	"testing"

	httpbind "github.com/shibukawa/tinybind-go"
	"github.com/shibukawa/tinybind-go/internal/bindcore"
)

func TestRestFormAny(t *testing.T) {
	for name, form := range map[string]httpbind.FormValues{
		"multipart map": bindcore.FormFromMap(map[string]string{"name": "n", "x": "1"}),
		// An urlencoded body is pairs; the first of a repeated key wins and
		// both halves are unescaped, as the map form's parser did.
		"urlencoded pairs": bindcore.ReadFormValues("name=n&x=1&x=2&y%20z=a+b"),
	} {
		t.Run(name, func(t *testing.T) {
			got := httpbind.RestFormAny(form, []string{"name"})
			if got["x"] != "1" {
				t.Fatalf("%#v", got)
			}
			if _, ok := got["name"]; ok {
				t.Fatalf("name in rest: %#v", got)
			}
			if name == "urlencoded pairs" && got["y z"] != "a b" {
				t.Fatalf("escaped pair not decoded: %#v", got)
			}
		})
	}
}
