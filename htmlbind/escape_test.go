package htmlbind

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// escapeCorpus pairs an input with the bytes both escapers must produce for it.
//
// The invalid-UTF-8 rows are the ones that matter. Escape decodes each invalid
// byte to the replacement character, and the fast path used to test only for
// the five entity characters — so the same bytes were sanitized or passed
// through depending on whether a '<' happened to be somewhere else in the
// value, which is a difference the value never asked for.
var escapeCorpus = []struct{ name, in, want string }{
	{"empty", "", ""},
	{"clean ascii", "hello world", "hello world"},
	{"clean utf8", "こんにちは", "こんにちは"},
	{"entities", `<a href="x">&'`, "&lt;a href=&#34;x&#34;&gt;&amp;&#39;"},
	{"genuine replacement character", "a�b", "a�b"},

	{"lone invalid byte", "\xff", "�"},
	{"invalid byte among ascii", "a\xffb", "a�b"},
	{"invalid byte beside an entity", "a\xffb<", "a�b&lt;"},
	{"invalid byte before an entity", "\xff&", "�&amp;"},
	{"truncated sequence", "a\xe3\x81", "a��"},
	{"surrogate encoding", "\xed\xa0\x80", "���"},
	{"overlong encoding", "\xc0\xaf", "��"},
	{"invalid byte after valid utf8", "あ\xff", "あ�"},
}

// TestEscapeSanitizesInvalidUTF8Consistently is the pair test: whether a value
// is rewritten cannot depend on a character elsewhere in it.
func TestEscapeSanitizesInvalidUTF8Consistently(t *testing.T) {
	for _, tc := range escapeCorpus {
		if got := Escape(tc.in); got != tc.want {
			t.Errorf("%s: Escape(%q) = %q, want %q", tc.name, tc.in, got, tc.want)
		}
	}
}

// WriteEscaped is the same rules written straight to the response, so it has to
// answer identically or a value renders one way through a helper and another
// through the instruction that actually emits it.
func TestWriteEscapedMatchesEscape(t *testing.T) {
	for _, tc := range escapeCorpus {
		var out strings.Builder
		r := &Renderer{w: &out, sw: &out}
		if err := r.WriteEscaped(tc.in); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got := out.String(); got != tc.want {
			t.Errorf("%s: WriteEscaped(%q) wrote %q, want %q", tc.name, tc.in, got, tc.want)
		}
	}
}

// Whatever comes out is valid UTF-8, which is the property the replacement
// exists for: a byte a decoder has to guess at is a byte an author cannot
// reason about.
func TestEscapeAlwaysProducesValidUTF8(t *testing.T) {
	for _, tc := range escapeCorpus {
		if got := Escape(tc.in); !utf8.ValidString(got) {
			t.Errorf("%s: Escape(%q) = %q, which is not valid UTF-8", tc.name, tc.in, got)
		}
	}
}

func FuzzEscapeMatchesWriteEscaped(f *testing.F) {
	for _, tc := range escapeCorpus {
		f.Add(tc.in)
	}
	f.Fuzz(func(t *testing.T, in string) {
		want := Escape(in)
		var out strings.Builder
		r := &Renderer{w: &out, sw: &out}
		if err := r.WriteEscaped(in); err != nil {
			t.Fatal(err)
		}
		if got := out.String(); got != want {
			t.Fatalf("in=%q: Escape = %q, WriteEscaped = %q", in, want, got)
		}
		if !utf8.ValidString(want) {
			t.Fatalf("in=%q: Escape = %q, which is not valid UTF-8", in, want)
		}
		// Ampersands survive as the entities' own leading character, so only
		// the four that can end or open markup are checked here; the corpus
		// test above pins the exact bytes for each of the five.
		if strings.ContainsAny(want, `<>"'`) {
			t.Fatalf("in=%q: Escape = %q, which still carries a markup character", in, want)
		}
	})
}
