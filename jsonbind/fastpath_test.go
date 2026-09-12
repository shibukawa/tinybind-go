package jsonbind

import (
	"encoding/json"
	"io"
	"math"
	"math/rand/v2"
	"strconv"
	"strings"
	"testing"
)

// The short-float encoder and the fast float reader each claim to produce
// what strconv produces. Both claims are checked here against strconv over
// the shapes they take: integers, short decimals, and random floats near
// the boundaries of each fast path.
func TestAppendFloatMatchesStrconvOnShortDecimals(t *testing.T) {
	check := func(v float64) {
		t.Helper()
		if math.IsInf(v, 0) || math.IsNaN(v) {
			return
		}
		want, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if got := AppendFloat(nil, v); string(got) != string(want) {
			t.Fatalf("AppendFloat(%v): got %s want %s", v, got, want)
		}
	}
	for i := -2000; i <= 2000; i++ {
		check(float64(i))
		check(float64(i) / 10)
		check(float64(i) / 100)
		check(float64(i) / 1000)
	}
	for _, v := range []float64{
		1e15 - 1, 1e15, 1e15 + 2, 1e9 - 0.5, 1e9, 1e9 + 0.5, 9007199254740992, 9007199254740993,
		0.1 + 0.2, 2.675, 1.005, 0.001, 0.0001, 123456.789, -0.5, math.Copysign(0, -1),
		math.Inf(0), math.NaN(),
	} {
		if math.IsInf(v, 0) || math.IsNaN(v) {
			// encoding/json refuses these; the encoder's fallback is strconv's
			// spelling, which is what the fallback wrote before.
			if got := AppendFloat(nil, v); string(got) != strconv.FormatFloat(v, 'f', -1, 64) {
				t.Fatalf("AppendFloat(%v): got %s", v, got)
			}
			continue
		}
		check(v)
	}
	r := rand.New(rand.NewPCG(1, 2))
	for i := 0; i < 200000; i++ {
		switch i % 4 {
		case 0:
			check(math.Float64frombits(r.Uint64()))
		case 1:
			check(float64(r.IntN(2000000000)-1000000000) / pow10[r.IntN(4)])
		case 2:
			check(r.Float64() * 1e9)
		default:
			check(math.Trunc(r.Float64() * 1e16))
		}
	}
}

func TestParseFloatFastMatchesStrconv(t *testing.T) {
	check := func(s string) {
		t.Helper()
		got, ok := parseFloatFast([]byte(s))
		if !ok {
			return
		}
		want, err := strconv.ParseFloat(s, 64)
		if err != nil {
			t.Fatal(err)
		}
		if got != want || math.Signbit(got) != math.Signbit(want) {
			t.Fatalf("parseFloatFast(%s): got %v want %v", s, got, want)
		}
	}
	for _, s := range []string{"0", "-0", "0.0", "-0.0", "1", "40.8", "-0.1", "312.23", "19.99", "9007199254740992", "9007199254740993", "0.1234567890123456789", "1234567890123456789", "12345678901234567890"} {
		check(s)
	}
	r := rand.New(rand.NewPCG(3, 4))
	for i := 0; i < 200000; i++ {
		whole := strconv.FormatUint(r.Uint64()%uint64(pow10[r.IntN(17)+1]), 10)
		s := whole
		if r.IntN(2) == 0 {
			s += "." + strconv.FormatUint(r.Uint64()%uint64(pow10[r.IntN(10)+1]), 10)
		}
		if r.IntN(2) == 0 {
			s = "-" + s
		}
		check(s)
	}
	// An exponent, or more digits than the fast path admits, is refused.
	for _, s := range []string{"1e3", "1E-3", "1.5e10", "12345678901234567890.5"} {
		if _, ok := parseFloatFast([]byte(s)); ok {
			t.Fatalf("parseFloatFast(%s) claimed the value", s)
		}
	}
}

// Repeated strings keep their values, and invalid UTF-8 is coerced each time.
func TestRepeatedStringsKeepTheirValues(t *testing.T) {
	var p Parser
	doc := []byte(`["gold","gold","未来","未来","x","x","` + strings.Repeat("a", 100) + `"]`)
	p.Reset(doc)
	if _, err := p.ArrayStart(); err != nil {
		t.Fatal(err)
	}
	var got []string
	for i := 0; ; i++ {
		more, err := p.ArrayNext(i)
		if err != nil {
			t.Fatal(err)
		}
		if !more {
			break
		}
		s, err := p.String()
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, s)
	}
	want := []string{"gold", "gold", "未来", "未来", "x", "x", strings.Repeat("a", 100)}
	if len(got) != len(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("value %d = %q, want %q", i, got[i], want[i])
		}
	}
	// Reuse the parser to verify invalid UTF-8 is coerced, not returned raw.
	p.Reset([]byte("[\"ab\x80cd\",\"ab\x80cd\"]"))
	if _, err := p.ArrayStart(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := p.ArrayNext(i); err != nil {
			t.Fatal(err)
		}
		s, err := p.String()
		if err != nil {
			t.Fatal(err)
		}
		if s != "ab�cd" {
			t.Fatalf("got %q", s)
		}
	}
}

// A type both registered and carrying a method is served by the registered
// function, which returns by value where the method needs its receiver boxed.
func TestRegisteredCodecWinsOverAMethod(t *testing.T) {
	RegisterDecode[bothWays](func([]byte) (bothWays, error) { return bothWays{Via: "registry"}, nil })
	RegisterEncode[bothWays](func(w io.Writer, v bothWays) error {
		_, err := w.Write([]byte(`"registry"`))
		return err
	})
	got, err := DecodeJSONBytes[bothWays]([]byte(`{}`))
	if err != nil || got.Via != "registry" {
		t.Fatalf("got %+v, %v", got, err)
	}
	var buf strings.Builder
	if err := EncodeJSON(&buf, bothWays{}); err != nil {
		t.Fatal(err)
	}
	if buf.String() != `"registry"` {
		t.Fatalf("got %s", buf.String())
	}
}

type bothWays struct{ Via string }

func (v bothWays) AppendJSONTo(dst []byte) []byte { return append(dst, `"method"`...) }
func (v *bothWays) DecodeJSONFrom([]byte) error   { v.Via = "method"; return nil }
