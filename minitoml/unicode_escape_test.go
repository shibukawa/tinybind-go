package minitoml

import "testing"

// The scaffold writer emits \uXXXX for control characters, so the reader has to
// accept it or a generated config cannot be loaded by the parser it was written
// for. \uXXXX and \UXXXXXXXX are TOML 1.0.0 escapes.
func TestUnicodeEscapes(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{`""`, ""},
		{`"ABC"`, "ABC"},
		{`"aéb"`, "aéb"},
		{`"\U0001F600"`, "\U0001F600"},
		{`"plain"`, "plain"},
		// The exact form configbind's scaffold writer emits: \u + uppercase hex.
		{"\"\\u001B\"", "\x1b"},
		{"\"\\u007F\"", "\x7f"},
	} {
		doc, err := ParseString("k = " + tc.in)
		if err != nil {
			t.Errorf("%s: %v", tc.in, err)
			continue
		}
		v, _ := doc.Get("k")
		if v.Str != tc.want {
			t.Errorf("%s decoded to %q, want %q", tc.in, v.Str, tc.want)
		}
	}
	// A short run, a non-hex digit, a surrogate, and a bare marker are errors
	// rather than a silently wrong rune.
	for _, bad := range []string{`"\u12"`, `"\uZZZZ"`, `"\uD800"`, `"\u"`} {
		if _, err := ParseString("k = " + bad); err == nil {
			t.Errorf("%s parsed, want an error", bad)
		}
	}
}
