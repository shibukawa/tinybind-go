package cachekeybind_test

import (
	"testing"
	"time"

	"github.com/shibukawa/tinybind-go/cachekeybind"
)

// The append helpers frame exactly as the string helpers do, so a key built
// either way reads the same entry.
func TestAppendHelpersFrameLikeTheStringHelpers(t *testing.T) {
	when := time.Date(2026, 9, 12, 10, 0, 0, 5, time.FixedZone("x", 3600))
	after := "a"
	for name, tc := range map[string]struct{ str, app string }{
		"string":   {cachekeybind.KeyString("abc"), string(cachekeybind.AppendKeyString(nil, "abc"))},
		"empty":    {cachekeybind.KeyString(""), string(cachekeybind.AppendKeyString(nil, ""))},
		"bytes":    {cachekeybind.KeyBytes([]byte{1, 2}), string(cachekeybind.AppendKeyBytes(nil, []byte{1, 2}))},
		"true":     {cachekeybind.KeyBool(true), string(cachekeybind.AppendKeyBool(nil, true))},
		"false":    {cachekeybind.KeyBool(false), string(cachekeybind.AppendKeyBool(nil, false))},
		"int":      {cachekeybind.KeyInt(int64(-9223372036854775808)), string(cachekeybind.AppendKeyInt(nil, int64(-9223372036854775808)))},
		"uint":     {cachekeybind.KeyUint(uint64(18446744073709551615)), string(cachekeybind.AppendKeyUint(nil, uint64(18446744073709551615)))},
		"float":    {cachekeybind.KeyFloat(float32(0.1)), string(cachekeybind.AppendKeyFloat(nil, float32(0.1)))},
		"negzero":  {cachekeybind.KeyFloat(-0.0), string(cachekeybind.AppendKeyFloat(nil, -0.0))},
		"time":     {cachekeybind.KeyTime(when), string(cachekeybind.AppendKeyTime(nil, when))},
		"absent":   {cachekeybind.KeyOptional[string](nil, cachekeybind.KeyString[string]), string(cachekeybind.AppendKeyOptional[string](nil, nil, cachekeybind.AppendKeyString[string]))},
		"present":  {cachekeybind.KeyOptional(&after, cachekeybind.KeyString[string]), string(cachekeybind.AppendKeyOptional(nil, &after, cachekeybind.AppendKeyString[string]))},
		"array":    {cachekeybind.KeyArray([]string{"a", "bc"}, cachekeybind.KeyString[string]), string(cachekeybind.AppendKeyArray(nil, []string{"a", "bc"}, cachekeybind.AppendKeyString[string]))},
		"nilarray": {cachekeybind.KeyArray[string](nil, cachekeybind.KeyString[string]), string(cachekeybind.AppendKeyArray[string](nil, nil, cachekeybind.AppendKeyString[string]))},
	} {
		if tc.str != tc.app {
			t.Errorf("%s: string helper %q, append helper %q", name, tc.str, tc.app)
		}
	}
}

func TestSizeHintSizesTheNextBuffer(t *testing.T) {
	var h cachekeybind.SizeHint
	first := h.Buffer()
	if cap(first) < 32 || len(first) != 0 {
		t.Fatalf("cap %d len %d", cap(first), len(first))
	}
	key := h.Finish(append(first, make([]byte, 100)...))
	if len(key) != 100 {
		t.Fatalf("len %d", len(key))
	}
	if next := h.Buffer(); cap(next) < 100 {
		t.Fatalf("hint did not grow: cap %d", cap(next))
	}
}
