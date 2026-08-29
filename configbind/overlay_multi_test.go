package configbind_test

import (
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/configbind"
)

// A list field reads through GetMulti. A real list from a repeated flag or a
// TOML array comes back verbatim; a scalar reaching a list field is env's
// comma-separated spelling of a list, which GetMulti splits, trims, and drops
// empties from — the behavior the doc comment always promised but the code did
// not do, so an env list arrived as one bogus element.
func TestGetMultiSplitsAScalarByComma(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want []string
	}{
		{"a.example,b.example", []string{"a.example", "b.example"}},
		{"a, b, c", []string{"a", "b", "c"}}, // spaces around commas are trimmed
		{"a,,b", []string{"a", "b"}},         // empty elements dropped
		{"a,b,", []string{"a", "b"}},         // trailing comma dropped
		{" solo ", []string{"solo"}},         // one value, trimmed
		{"", nil},                            // an empty scalar is an empty list
	} {
		o := configbind.NewOverlay()
		o.Set("list", tc.raw, configbind.PlaceEnv)
		got, ok := o.GetMulti("list")
		if !ok {
			t.Errorf("%q: GetMulti reported absent", tc.raw)
			continue
		}
		if strings.Join(got, "|") != strings.Join(tc.want, "|") {
			t.Errorf("GetMulti(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

// A real multi-value — the shape a repeated CLI flag and a TOML array produce —
// is returned exactly, with no split and no trim, so a value that legitimately
// holds a comma or a space survives when it came from a source that can express
// a list.
func TestGetMultiLeavesARealListAlone(t *testing.T) {
	o := configbind.NewOverlay()
	o.SetMulti("list", []string{"a,b", " spaced "}, configbind.PlaceCLI)
	got, ok := o.GetMulti("list")
	if !ok {
		t.Fatal("GetMulti reported absent")
	}
	if len(got) != 2 || got[0] != "a,b" || got[1] != " spaced " {
		t.Fatalf("GetMulti = %q, want the multi-value verbatim", got)
	}
}
