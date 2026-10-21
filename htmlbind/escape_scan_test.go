package htmlbind

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

// escapeRuneLoop is the Escape the word scan replaced: a library prefilter and
// a rune loop. It stays as the oracle for Escape and WriteEscaped, so the scan
// is checked against the rules themselves rather than against the examples
// in escapeCorpus alone.
func escapeRuneLoop(value string) string {
	if !strings.ContainsAny(value, `&<>"'`) && utf8.ValidString(value) {
		return value
	}
	var out strings.Builder
	start := 0
	for i, r := range value {
		var entity string
		switch r {
		case '&':
			entity = "&amp;"
		case '<':
			entity = "&lt;"
		case '>':
			entity = "&gt;"
		case '"':
			entity = "&#34;"
		case '\'':
			entity = "&#39;"
		case utf8.RuneError:
			if _, width := utf8.DecodeRuneInString(value[i:]); width != 1 {
				continue
			}
			entity = "�"
		default:
			continue
		}
		out.WriteString(value[start:i])
		out.WriteString(entity)
		start = i + 1
	}
	out.WriteString(value[start:])
	return out.String()
}

// appendJSONStringByteLoop is the AppendJSONString the word scan replaced,
// kept as its oracle for the same reason.
func appendJSONStringByteLoop[T ~string | ~[]byte](dst []byte, value T) []byte {
	const hex = "0123456789abcdef"
	dst = append(dst, '"')
	start := 0
	for i := 0; i < len(value); {
		c := value[i]
		if c < utf8.RuneSelf {
			if c >= 0x20 && c != '"' && c != '\\' && c != '<' && c != '>' && c != '&' {
				i++
				continue
			}
			dst = append(dst, value[start:i]...)
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
		end := min(i+utf8.UTFMax, len(value))
		r, width := utf8.DecodeRuneInString(string(value[i:end]))
		switch {
		case r == ' ', r == ' ':
			dst = append(dst, value[start:i]...)
			if r == ' ' {
				dst = append(dst, '\\', 'u', '2', '0', '2', '8')
			} else {
				dst = append(dst, '\\', 'u', '2', '0', '2', '9')
			}
			i += width
			start = i
		case r == utf8.RuneError && width == 1:
			dst = append(dst, value[start:i]...)
			dst = append(dst, "�"...)
			i += width
			start = i
		default:
			i += width
		}
	}
	dst = append(dst, value[start:]...)
	return append(dst, '"')
}

// scanCorpus places every kind of stop at every offset of a word, on both
// sides of the eight-byte boundary, so a lane the mask marks late or a tail
// the word loop leaves behind shows up as a mismatch.
func scanCorpus() []string {
	stops := []string{
		`&`, `<`, `>`, `"`, `'`, `\`, "\n", "\x00", "\x1f", "\x7f", "#",
		"é", "日", "😀", "�", " ", " ",
		"\x80", "\xff", "\xe2\x80", "\xed\xa0\x80", "\xc0\xaf",
	}
	var out []string
	for width := 0; width <= 17; width++ {
		plain := strings.Repeat("a", width)
		out = append(out, plain)
		for at := 0; at <= width; at++ {
			for _, stop := range stops {
				out = append(out, plain[:at]+stop+plain[at:])
			}
		}
	}
	for _, tc := range escapeCorpus {
		out = append(out, tc.in)
	}
	out = append(out,
		"東京都千代田区千代田一番一号",
		"日本語 and ASCII, with \"quotes\" & <tags> at the 'end'",
		strings.Repeat("plain ", 20)+`"`+strings.Repeat("x", 20),
		strings.Repeat("日本語", 10)+"\x80"+strings.Repeat("abcdefgh", 3),
		strings.Repeat(`<td class="cell">value</td>`, 4),
	)
	return out
}

func checkEscape(t *testing.T, in string) {
	t.Helper()
	want := escapeRuneLoop(in)
	if got := Escape(in); got != want {
		t.Fatalf("Escape(%q):\n got %q\nwant %q", in, got, want)
	}
	var out strings.Builder
	r := &Renderer{w: &out, sw: &out}
	if err := r.WriteEscaped(in); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != want {
		t.Fatalf("WriteEscaped(%q):\n got %q\nwant %q", in, got, want)
	}
}

func checkAppendJSONString(t *testing.T, in string) {
	t.Helper()
	want := string(appendJSONStringByteLoop(nil, in))
	if got := string(AppendJSONString(nil, in)); got != want {
		t.Fatalf("AppendJSONString(%q):\n got %s\nwant %s", in, got, want)
	}
	if got := string(AppendJSONString(nil, []byte(in))); got != want {
		t.Fatalf("AppendJSONString([]byte(%q)):\n got %s\nwant %s", in, got, want)
	}
	if utf8.ValidString(in) {
		std, err := json.Marshal(in)
		if err != nil {
			t.Fatal(err)
		}
		if want != string(std) {
			t.Fatalf("AppendJSONString(%q) against encoding/json:\n got %s\nwant %s", in, want, std)
		}
	}
}

func TestEscapeMatchesTheRuneLoop(t *testing.T) {
	for _, in := range scanCorpus() {
		checkEscape(t, in)
	}
}

func TestAppendJSONStringMatchesTheByteLoop(t *testing.T) {
	for _, in := range scanCorpus() {
		checkAppendJSONString(t, in)
	}
}

func FuzzEscapeMatchesTheRuneLoop(f *testing.F) {
	for _, in := range scanCorpus() {
		f.Add(in)
	}
	f.Fuzz(checkEscape)
}

func FuzzAppendJSONStringMatchesTheByteLoop(f *testing.F) {
	for _, in := range scanCorpus() {
		f.Add(in)
	}
	f.Fuzz(checkAppendJSONString)
}
