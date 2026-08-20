package htmlbind

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// legacyAppendJSONString is the escaper exactly as it stood before
// AppendJSONString was exported. It is kept here so the corpus below can prove
// the export moved no bytes, which is the one thing a caller already embedding
// this output in a script element cannot re-verify for itself.
func legacyAppendJSONString(dst []byte, value string) []byte {
	const hex = "0123456789abcdef"
	if free := cap(dst) - len(dst); free < len(value)+2 {
		grown := make([]byte, len(dst), len(dst)+len(value)+18)
		copy(grown, dst)
		dst = grown
	}
	dst = append(dst, '"')
	start := 0
	for i := 0; i < len(value); {
		c := value[i]
		if c < utf8.RuneSelf {
			if c >= 0x20 && c != '"' && c != '\\' && c != '<' && c != '>' && c != '&' {
				i++
				continue
			}
			if start < i {
				dst = append(dst, value[start:i]...)
			}
			switch c {
			case '"':
				dst = append(dst, '\\', '"')
			case '\\':
				dst = append(dst, '\\', '\\')
			case '\b':
				dst = append(dst, '\\', 'b')
			case '\f':
				dst = append(dst, '\\', 'f')
			case '\n':
				dst = append(dst, '\\', 'n')
			case '\r':
				dst = append(dst, '\\', 'r')
			case '\t':
				dst = append(dst, '\\', 't')
			case '<':
				dst = append(dst, '\\', 'u', '0', '0', '3', 'c')
			case '>':
				dst = append(dst, '\\', 'u', '0', '0', '3', 'e')
			case '&':
				dst = append(dst, '\\', 'u', '0', '0', '2', '6')
			default:
				dst = append(dst, '\\', 'u', '0', '0', hex[c>>4], hex[c&15])
			}
			i++
			start = i
			continue
		}
		r, width := utf8.DecodeRuneInString(value[i:])
		switch {
		case r == ' ', r == ' ':
			if start < i {
				dst = append(dst, value[start:i]...)
			}
			if r == ' ' {
				dst = append(dst, '\\', 'u', '2', '0', '2', '8')
			} else {
				dst = append(dst, '\\', 'u', '2', '0', '2', '9')
			}
			i += width
			start = i
		case r == utf8.RuneError && width == 1:
			// An invalid byte becomes the replacement character, exactly as a
			// rune loop would decode it. A genuine U+FFFD is three valid bytes
			// and travels inside the clean run.
			if start < i {
				dst = append(dst, value[start:i]...)
			}
			dst = append(dst, "�"...)
			i += width
			start = i
		default:
			i += width
		}
	}
	if start < len(value) {
		dst = append(dst, value[start:]...)
	}
	return append(dst, '"')
}

// jsonStringCorpus reaches every branch of the escaper: the bulk-copy path, all
// three of the HTML-closing escapes, both line separators, each named control
// escape and the numeric fallback for the rest, a genuine U+FFFD travelling
// inside a clean run, a truncated sequence of every width, and a fragment long
// enough that the clean runs between escapes are worth copying in bulk.
func jsonStringCorpus() []string {
	corpus := []string{
		"",
		"plain ascii",
		`"`, `\`, "<", ">", "&",
		"\b", "\f", "\n", "\r", "\t", "\x00", "\x1f", "\x7f",
		"\u2028", "\u2029", "a\u2028b\u2029c",
		"\uFFFD", "before\uFFFDafter",
		"\xff", "abc\xff", "\xff\xfe\xfd",
		"\xe4\xb8", "\xf0\x9f", "\xf0\x9f\x8e",
		"\u65e5\u672c\u8a9e", "\U0001f389", "mixed \u65e5 <b>&</b> \U0001f389",
		"<script>alert(\"x\" & '\\'')</script>",
		strings.Repeat(`<td class="cell">value & more</td>`, 128),
		strings.Repeat("clean", 2048),
	}
	for b := 0; b < 0x80; b++ {
		corpus = append(corpus, string([]byte{byte(b)}), "pre"+string([]byte{byte(b)})+"post")
	}
	return corpus
}

func TestAppendJSONStringMovesNoBytes(t *testing.T) {
	for _, input := range jsonStringCorpus() {
		want := string(legacyAppendJSONString(nil, input))
		if got := string(AppendJSONString(nil, input)); got != want {
			t.Errorf("AppendJSONString(%q) = %q, escaper it replaced = %q", input, got, want)
		}
		if got := JSONString(input); got != want {
			t.Errorf("JSONString(%q) = %q, escaper it replaced = %q", input, got, want)
		}
	}
}

func TestAppendJSONStringAgreesWithJSONString(t *testing.T) {
	for _, input := range jsonStringCorpus() {
		want := JSONString(input)
		if got := string(AppendJSONString(nil, input)); got != want {
			t.Errorf("AppendJSONString(%q) = %q, JSONString = %q", input, got, want)
		}
		if got := string(AppendJSONString(nil, []byte(input))); got != want {
			t.Errorf("AppendJSONString([]byte(%q)) = %q, JSONString = %q", input, got, want)
		}
	}
}

// A named string type and a named byte-slice type reach the same escaper the
// plain kinds do, which is what lets a caller pass TrustedHTML or a rendered
// fragment without converting first.
func TestAppendJSONStringTakesNamedTypes(t *testing.T) {
	type namedBytes []byte
	const input = `<b>café</b> & "more"`
	want := JSONString(input)
	if got := string(AppendJSONString(nil, TrustedHTML(input))); got != want {
		t.Errorf("AppendJSONString(TrustedHTML) = %q, want %q", got, want)
	}
	if got := string(AppendJSONString(nil, namedBytes(input))); got != want {
		t.Errorf("AppendJSONString(namedBytes) = %q, want %q", got, want)
	}
}

// Appending extends what dst already holds rather than replacing it, which is
// the whole point of the entry: a caller assembles a record around the call.
func TestAppendJSONStringExtendsDst(t *testing.T) {
	dst := AppendJSONString([]byte(`{"id":`), "b-1")
	dst = append(dst, `,"html":`...)
	dst = AppendJSONString(dst, []byte("<p>hi</p>"))
	if got, want := string(append(dst, '}')), `{"id":"b-1","html":"\u003cp\u003ehi\u003c/p\u003e"}`; got != want {
		t.Errorf("assembled record = %q, want %q", got, want)
	}
}

// A short dst is grown rather than overrun, so a nil scratch stays legal.
func TestAppendJSONStringGrowsAShortDst(t *testing.T) {
	for _, dst := range [][]byte{nil, make([]byte, 0, 1), make([]byte, 3, 4)} {
		prefix := string(dst)
		got := string(AppendJSONString(dst, strings.Repeat("<x>", 64)))
		if want := prefix + JSONString(strings.Repeat("<x>", 64)); got != want {
			t.Errorf("AppendJSONString(cap %d) = %q, want %q", cap(dst), got, want)
		}
	}
}

// allocSink escapes so that the probe below allocates on any toolchain that
// counts allocations at all.
var allocSink []byte

// allocationsAreCounted reports whether this toolchain counts allocations at
// all. TinyGo's AllocsPerRun returns zero for an allocation that plainly
// escapes, so a zero from it means nothing was measured rather than nothing was
// allocated, and asserting on it would leave the tests below passing for no
// reason.
//
// Callers log and return rather than calling t.Skip, because TinyGo's SkipNow
// does not call runtime.Goexit: a test skipped there runs on to the assertion
// it was skipping and is reported as a failure.
func allocationsAreCounted() bool {
	return testing.AllocsPerRun(10, func() { allocSink = make([]byte, 4096) }) > 0
}

const uncounted = "this toolchain does not count allocations; nothing asserted"

// The entry exists so a caller assembling into a reused buffer stops paying per
// field. A []byte fragment appended into a buffer with room costs nothing;
// JSONString on the same fragment costs three allocations.
func TestAppendJSONStringAppendsWithoutAllocating(t *testing.T) {
	if !allocationsAreCounted() {
		t.Log(uncounted)
		return
	}
	fragment := []byte(strings.Repeat(`<td class="cell">value</td>`, 32))
	scratch := make([]byte, 0, 8192)
	if allocs := testing.AllocsPerRun(200, func() {
		_ = AppendJSONString(scratch[:0], fragment)
	}); allocs != 0 {
		t.Errorf("AppendJSONString into a pre-grown buffer allocated %v times, want 0", allocs)
	}
	if allocs := testing.AllocsPerRun(200, func() {
		_ = append(scratch[:0], JSONString(string(fragment))...)
	}); allocs == 0 {
		t.Error("JSONString(string(fragment)) allocated nothing; the corpus no longer measures what the entry removes")
	}
}

// Content.AppendJSON is the module's own record writer and holds its fragment
// as bytes, so it is the first caller the export was meant to relieve.
func TestContentAppendJSONEscapesItsFragmentWithoutConverting(t *testing.T) {
	content := Content{BoundaryID: "tb-3", HTML: []byte(strings.Repeat("<span>x</span>", 32))}
	want := `{"id":` + JSONString(content.BoundaryID) + `,"html":` + JSONString(string(content.HTML)) + `}`
	if got := string(content.AppendJSON(nil)); got != want {
		t.Errorf("Content.AppendJSON = %q, want %q", got, want)
	}
	if !allocationsAreCounted() {
		t.Log(uncounted)
		return
	}
	scratch := make([]byte, 0, 8192)
	if allocs := testing.AllocsPerRun(200, func() {
		_ = content.AppendJSON(scratch[:0])
	}); allocs != 0 {
		t.Errorf("Content.AppendJSON into a pre-grown buffer allocated %v times, want 0", allocs)
	}
}
