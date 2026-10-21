package jsonbind

import (
	"strings"
	"testing"
)

var benchBytes []byte

// BenchmarkAppendString covers the shapes a response carries: short
// identifiers, an address, a sentence, a long plain run, a value with escapes
// inside it, and non-ASCII text, which takes the rune path throughout.
func BenchmarkAppendString(b *testing.B) {
	cases := []struct{ name, s string }{
		{"short", "SKU-1"},
		{"email", "ada@example.com"},
		{"sentence", "leave at the front desk, ring twice"},
		{"long", strings.Repeat("plain ascii text ", 8)},
		{"escaped", "https://example.com/search?q=a&b=<c>"},
		{"japanese", "東京都千代田区千代田一番一号の住所です"},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			buf := make([]byte, 0, 1024)
			b.ReportAllocs()
			for b.Loop() {
				buf = AppendString(buf[:0], c.s)
			}
			benchBytes = buf
		})
	}
}
