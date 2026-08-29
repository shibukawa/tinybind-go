package minitoml

import (
	"strings"
	"testing"
)

// TOML 1.0.0 makes defining a key twice invalid, and every mainstream parser
// rejects it; last-wins was minitoml's alone. These are the exact-duplicate
// shapes that must now error rather than silently keep the later value.
func TestDuplicateKeyRejected(t *testing.T) {
	for _, src := range []string{
		"port = 1\nport = 2",
		"a.b = 1\na.b = 2",
		"[server]\nport = 1\nport = 2",
		"[a]\nb = 1\nb = 2",
		"x = 1\n[t]\nx = 2\nx = 3",
		"[[servers]]\nname = \"a\"\nname = \"b\"", // same element, duplicated
	} {
		if _, err := Parse([]byte(src)); err == nil {
			t.Errorf("duplicate key accepted:\n%s", src)
		} else if !strings.Contains(err.Error(), "defined") && !strings.Contains(err.Error(), "duplicate") {
			t.Errorf("wrong error for a duplicate key: %v\nsource:\n%s", err, src)
		}
	}
}

// The legal shapes that share a prefix or a table name but define no key twice
// must still parse — the check must not mistake a nested or dotted key, a second
// table, or another array element for a redefinition.
func TestDuplicateKeyCheckKeepsLegalTOML(t *testing.T) {
	for _, src := range []string{
		"a.b = 1\na.c = 2",
		"a.b.c = 1\na.b.d = 2",
		"[x]\na = 1\n[y]\na = 2",
		"[server]\na.b = 1\na.c = 2",
		"[a]\n[a.b]\nc = 1",
		"[[servers]]\nname = \"a\"\n[[servers]]\nname = \"b\"",
		"[[servers]]\nport = 1\n[[servers]]\nport = 2",
		"first = 1\nsecond = 2\nthird = 3",
	} {
		if _, err := Parse([]byte(src)); err != nil {
			t.Errorf("legal TOML rejected: %v\nsource:\n%s", err, src)
		}
	}
}
