package htmlbind

import (
	"io"
	"strings"
	"testing"
)

var (
	benchString string
	benchBytes  []byte
)

// escapeBenchCases are the shapes a page's values take: short attribute
// values, a name, an address, a sentence, a long plain run, values carrying
// an entity, and non-ASCII text, which takes the rune path throughout.
var escapeBenchCases = []struct{ name, s string }{
	{"short", "Ada"},
	{"name", "Ada Lovelace"},
	{"email", "ada@example.com"},
	{"sentence", "leave at the front desk, ring twice"},
	{"long", strings.Repeat("plain ascii text ", 8)},
	{"entity", "Team & Co <staff>"},
	{"long-entity", strings.Repeat("plain ascii text ", 8) + "& more"},
	{"japanese", "東京都千代田区千代田一番一号の住所です"},
}

func BenchmarkEscape(b *testing.B) {
	for _, c := range escapeBenchCases {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				benchString = Escape(c.s)
			}
		})
	}
}

func BenchmarkWriteEscaped(b *testing.B) {
	for _, c := range escapeBenchCases {
		b.Run(c.name, func(b *testing.B) {
			r := &Renderer{w: io.Discard, sw: io.Discard.(io.StringWriter)}
			b.ReportAllocs()
			for b.Loop() {
				if err := r.WriteEscaped(c.s); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkAppendJSONString(b *testing.B) {
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
				buf = AppendJSONString(buf[:0], c.s)
			}
			benchBytes = buf
		})
	}
	// A rendered fragment is bytes, the other member of the type set.
	fragment := []byte(strings.Repeat(`<td class="cell">value</td>`, 32))
	b.Run("fragment-bytes", func(b *testing.B) {
		buf := make([]byte, 0, 8192)
		b.ReportAllocs()
		for b.Loop() {
			buf = AppendJSONString(buf[:0], fragment)
		}
		benchBytes = buf
	})
}
