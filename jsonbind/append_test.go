package jsonbind

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

// appendStringByteLoop is the byte-at-a-time escaper AppendString replaced. It
// stays as the oracle for the word scan: the two must agree on every input,
// including invalid UTF-8, where encoding/json's own spelling depends on the
// toolchain and cannot serve.
func appendStringByteLoop(dst []byte, s string) []byte {
	dst = append(dst, '"')
	start := 0
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			if c >= 0x20 && c != '"' && c != '\\' && c != '<' && c != '>' && c != '&' {
				i++
				continue
			}
			dst = append(dst, s[start:i]...)
			switch c {
			case '"':
				dst = append(dst, '\\', '"')
			case '\\':
				dst = append(dst, '\\', '\\')
			case '\n':
				dst = append(dst, '\\', 'n')
			case '\r':
				dst = append(dst, '\\', 'r')
			case '\t':
				dst = append(dst, '\\', 't')
			case '\b':
				dst = append(dst, '\\', 'b')
			case '\f':
				dst = append(dst, '\\', 'f')
			default:
				dst = append(dst, '\\', 'u', '0', '0', hexDigits[c>>4], hexDigits[c&0xF])
			}
			i++
			start = i
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			dst = append(dst, s[start:i]...)
			dst = append(dst, '\\', 'u', 'f', 'f', 'f', 'd')
			i += size
			start = i
			continue
		}
		if r == '\u2028' || r == '\u2029' {
			dst = append(dst, s[start:i]...)
			dst = append(dst, '\\', 'u', '2', '0', '2', hexDigits[r&0xF])
			i += size
			start = i
			continue
		}
		i += size
	}
	dst = append(dst, s[start:]...)
	return append(dst, '"')
}

// appendStringCorpus places every kind of stop at every offset of a word, on
// both sides of the eight-byte boundary, so a lane the mask marks late or a
// tail the word loop leaves behind shows up as a mismatch.
func appendStringCorpus() []string {
	stops := []string{`"`, `&`, `<`, `>`, `\\`, "\n", "\x00", "\x1f", "\x7f", "é", "日", "😀", "\u2028", "\u2029", "\x80", "\xe2\x80"}
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
	out = append(out,
		"", "plain", `quote" backslash\ slash/`,
		"控制\x00\x01\x1f", "tab\tnl\ncr\rbs\bff\f",
		"html <b>&amp;</b>", "unicode é日😀",
		"js separators \u2028 \u2029",
		"東京都千代田区千代田一番一号",
		"日本語 and ASCII, with \"quotes\" & <tags> at the end",
		strings.Repeat("plain ", 20)+`"`+strings.Repeat("x", 20),
		strings.Repeat("日本語", 10)+"\x80"+strings.Repeat("abcdefgh", 3),
	)
	return out
}

func TestAppendStringMatchesTheByteLoop(t *testing.T) {
	for _, s := range appendStringCorpus() {
		got := string(AppendString(nil, s))
		want := string(appendStringByteLoop(nil, s))
		if got != want {
			t.Fatalf("AppendString(%q):\n got %s\nwant %s", s, got, want)
		}
		if utf8.ValidString(s) {
			std, err := json.Marshal(s)
			if err != nil {
				t.Fatal(err)
			}
			if got != string(std) {
				t.Fatalf("AppendString(%q) against encoding/json:\n got %s\nwant %s", s, got, std)
			}
		}
	}
}

func FuzzAppendStringMatchesTheByteLoop(f *testing.F) {
	for _, s := range appendStringCorpus() {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		got := string(AppendString(nil, s))
		want := string(appendStringByteLoop(nil, s))
		if got != want {
			t.Fatalf("AppendString(%q):\n got %s\nwant %s", s, got, want)
		}
		if utf8.ValidString(s) {
			std, err := json.Marshal(s)
			if err != nil {
				t.Fatal(err)
			}
			if got != string(std) {
				t.Fatalf("AppendString(%q) against encoding/json:\n got %s\nwant %s", s, got, std)
			}
		}
	})
}
