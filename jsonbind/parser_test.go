package jsonbind

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"
)

// decodeDoc drives the parser the way generated code does, so these tests
// exercise the shape the emitter actually produces.
type doc struct {
	Name   string
	Count  int
	Big    int64
	Ratio  float64
	OK     bool
	Tags   []string
	Nested map[string]string
	Extra  map[string]any
}

func decodeDoc(data []byte) (doc, error) {
	var out doc
	out.Extra = map[string]any{}
	p := NewParser(data)
	null, err := p.ObjectStart()
	if err != nil || null {
		return out, err
	}
	for n := 0; ; n++ {
		key, ok, err := p.ObjectKey(n)
		if err != nil {
			return out, err
		}
		if !ok {
			return out, p.End()
		}
		switch string(key) {
		case "name":
			out.Name, err = p.String()
		case "count":
			out.Count, err = p.Int()
		case "big":
			out.Big, err = p.Int64()
		case "ratio":
			out.Ratio, err = p.Float64()
		case "ok":
			out.OK, err = p.Bool()
		case "tags":
			var null bool
			if null, err = p.ArrayStart(); err == nil && !null {
				out.Tags = []string{}
				for i := 0; ; i++ {
					var more bool
					if more, err = p.ArrayNext(i); err != nil || !more {
						break
					}
					var s string
					if s, err = p.String(); err != nil {
						break
					}
					out.Tags = append(out.Tags, s)
				}
			}
		case "nested":
			out.Nested, err = DecodeJSONMapStringString(mustRaw(p))
		default:
			var v any
			if v, err = p.Any(); err == nil {
				out.Extra[string(key)] = v
			}
		}
		if err != nil {
			return out, err
		}
	}
}

func mustRaw(p *Parser) []byte {
	raw, err := p.RawValue()
	if err != nil {
		return nil
	}
	return raw
}

func TestParser_DecodesEveryScalarShape(t *testing.T) {
	got, err := decodeDoc([]byte(`{
		"name": "Ada é😀\t\"q\"",
		"count": -42,
		"big": 9007199254740993,
		"ratio": -1.5e-7,
		"ok": true,
		"tags": ["a", "b"],
		"nested": {"k": "v"},
		"other": {"deep": [1, null, false]}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Ada é😀\t\"q\"" {
		t.Fatalf("name: %q", got.Name)
	}
	if got.Count != -42 || got.Big != 9007199254740993 || got.Ratio != -1.5e-7 || !got.OK {
		t.Fatalf("scalars: %+v", got)
	}
	if !reflect.DeepEqual(got.Tags, []string{"a", "b"}) {
		t.Fatalf("tags: %#v", got.Tags)
	}
	if !reflect.DeepEqual(got.Nested, map[string]string{"k": "v"}) {
		t.Fatalf("nested: %#v", got.Nested)
	}
	want := map[string]any{"deep": []any{float64(1), nil, false}}
	if !reflect.DeepEqual(got.Extra["other"], want) {
		t.Fatalf("extra: %#v", got.Extra["other"])
	}
}

func TestParser_NullDecodesAsZero(t *testing.T) {
	got, err := decodeDoc([]byte(`{"name":null,"count":null,"ratio":null,"ok":null,"tags":null}`))
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "" || got.Count != 0 || got.Ratio != 0 || got.OK || got.Tags != nil {
		t.Fatalf("null handling: %+v", got)
	}
}

func TestParser_RejectsMalformed(t *testing.T) {
	for _, in := range []string{
		`{`,
		`{"name"}`,
		`{"name":}`,
		`{"name":"a"`,
		`{"name":"a",}`,
		`{"name":"unterminated}`,
		`{"count":"not a number"}`,
		`{"count":1.5}`,   // encoding/json rejects a fraction for an int too
		`{"count":1e3}`,   // and an exponent
		`{"ok":"true"}`,   // a quoted boolean is not a boolean
		`{"name":"a"} {}`, // trailing document
		`[1,2]`,
		`{"tags":[1,]}`,
	} {
		if _, err := decodeDoc([]byte(in)); err == nil {
			t.Fatalf("expected rejection of %s", in)
		}
	}
}

func TestParser_IntRangeMatchesStdlib(t *testing.T) {
	for _, in := range []string{
		`{"big":9223372036854775807}`,
		`{"big":-9223372036854775808}`,
	} {
		if _, err := decodeDoc([]byte(in)); err != nil {
			t.Fatalf("%s: %v", in, err)
		}
	}
	for _, in := range []string{
		`{"big":9223372036854775808}`,
		`{"big":-9223372036854775809}`,
	} {
		if _, err := decodeDoc([]byte(in)); err == nil {
			t.Fatalf("expected overflow rejection of %s", in)
		}
	}
}

func TestParser_InvalidUTF8BecomesReplacementChar(t *testing.T) {
	// Matches what encoding/json does with an unpaired continuation byte.
	in := []byte(`{"name":"` + "\x80" + `"}`)
	got, err := decodeDoc(in)
	if err != nil {
		t.Fatal(err)
	}
	var want struct{ Name string }
	if err := json.Unmarshal(in, &want); err != nil {
		t.Fatal(err)
	}
	if got.Name != want.Name {
		t.Fatalf("got %q want %q", got.Name, want.Name)
	}
}

func TestParser_EscapedKeyStaysValidWhileValueIsRead(t *testing.T) {
	// The key and value scratch buffers must be separate, or an escaped key
	// would be clobbered by the escaped value that follows it.
	p := NewParser([]byte(`{"abc":"xyz"}`))
	if _, err := p.ObjectStart(); err != nil {
		t.Fatal(err)
	}
	key, ok, err := p.ObjectKey(0)
	if err != nil || !ok {
		t.Fatalf("key: %v ok=%v", err, ok)
	}
	value, err := p.String()
	if err != nil {
		t.Fatal(err)
	}
	if string(key) != "abc" {
		t.Fatalf("key clobbered by value decode: %q", key)
	}
	if value != "xyz" {
		t.Fatalf("value: %q", value)
	}
}

func TestParser_SkipValueValidatesStructure(t *testing.T) {
	p := NewParser([]byte(`{"a":[1,{"b":[null,true]},"s"],"c":1}`))
	if _, err := p.ObjectStart(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := p.ObjectKey(0); err != nil {
		t.Fatal(err)
	}
	if err := p.SkipValue(); err != nil {
		t.Fatalf("skip: %v", err)
	}
	key, ok, err := p.ObjectKey(1)
	if err != nil || !ok || string(key) != "c" {
		t.Fatalf("parser lost its place after skip: key=%q ok=%v err=%v", key, ok, err)
	}
}


func TestReadLimitHint_ReadsExactlyToTheLimit(t *testing.T) {
	body := `{"value":"12345"}`
	got, err := ReadLimitHint(strings.NewReader(body), int64(len(body)), 0)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != body {
		t.Fatalf("got %q", got)
	}
	if _, err := ReadLimitHint(strings.NewReader(body), int64(len(body))-1, 0); err != ErrBodyTooLarge {
		t.Fatalf("want ErrBodyTooLarge, got %v", err)
	}
}

func TestReadLimitHint_WrongHintStillReadsEverything(t *testing.T) {
	body := strings.Repeat("x", 5000)
	for _, hint := range []int64{0, 1, 10, 5000, 1 << 20} {
		got, err := ReadLimitHint(strings.NewReader(body), 1<<20, hint)
		if err != nil {
			t.Fatalf("hint %d: %v", hint, err)
		}
		if len(got) != len(body) {
			t.Fatalf("hint %d: read %d of %d bytes", hint, len(got), len(body))
		}
	}
}

func TestAppend_MatchesEncodingJSON(t *testing.T) {
	for _, s := range []string{
		"", "plain", `quote" backslash\ slash/`,
		"控制\x00\x01\x1f", "tab\tnl\ncr\rbs\bff\f",
		"html <b>&amp;</b>", "unicode é日😀",
		"js separators   ",
	} {
		want, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		if got := AppendString(nil, s); string(got) != string(want) {
			t.Fatalf("AppendString(%q):\n got %s\nwant %s", s, got, want)
		}
	}
	// Invalid UTF-8 is pinned rather than compared: both spellings decode to
	// U+FFFD, and which one encoding/json writes depends on GOEXPERIMENT=jsonv2.
	// jsonbind writes the escape, which is what the default v1 encoder produces.
	if got, want := string(AppendString(nil, "invalid \x80 utf8")), `"invalid \ufffd utf8"`; got != want {
		t.Fatalf("AppendString on invalid UTF-8:\n got %s\nwant %s", got, want)
	}
	for _, f := range []float64{
		0, 1, -1, 0.5, 1e20, 1e21, 1e-6, 1e-7, 1.0000000000000002,
		math.SmallestNonzeroFloat64, math.MaxFloat64, -math.MaxFloat64,
	} {
		want, err := json.Marshal(f)
		if err != nil {
			t.Fatal(err)
		}
		if got := AppendFloat(nil, f); string(got) != string(want) {
			t.Fatalf("AppendFloat(%v):\n got %s\nwant %s", f, got, want)
		}
	}
}

func TestSortedKeys_IsDeterministic(t *testing.T) {
	m := map[string]int{"b": 1, "a": 2, "c": 3, "": 4, "ab": 5}
	want := []string{"", "a", "ab", "b", "c"}
	for range 20 {
		if got := SortedKeys(m); !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v want %#v", got, want)
		}
	}
}

func TestAppendAny_CoversParserAnyShapes(t *testing.T) {
	v := map[string]any{
		"s": "x", "n": float64(1.5), "b": true, "z": nil,
		"a": []any{float64(1), "two", nil},
		"o": map[string]any{"k": "v"},
	}
	want, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if got := AppendAny(nil, v); string(got) != string(want) {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
}

// FuzzParserMatchesEncodingJSON pins the parser to encoding/json's behaviour on
// documents stdlib accepts. Names are matched exactly here — unlike
// encoding/json, which also accepts a case-insensitive match — so the corpus
// sticks to lower-case names.
func FuzzParserMatchesEncodingJSON(f *testing.F) {
	f.Add(`{"name":"a","count":1,"ratio":0.5,"ok":true,"tags":["x"]}`)
	f.Add(`{}`)
	f.Add(`{"name":"é😀"}`)
	f.Add(`  {  "count" : -3 }  `)
	f.Fuzz(func(t *testing.T, in string) {
		var want struct {
			Name  string   `json:"name"`
			Count int      `json:"count"`
			Ratio float64  `json:"ratio"`
			OK    bool     `json:"ok"`
			Tags  []string `json:"tags"`
		}
		if err := json.Unmarshal([]byte(in), &want); err != nil {
			t.Skip()
		}
		if hasNonCanonicalName(in) {
			t.Skip()
		}
		got, err := decodeDoc([]byte(in))
		if err != nil {
			t.Fatalf("stdlib accepted %q, parser rejected it: %v", in, err)
		}
		if got.Name != want.Name || got.Count != want.Count || got.Ratio != want.Ratio ||
			got.OK != want.OK || !reflect.DeepEqual(got.Tags, want.Tags) {
			t.Fatalf("mismatch for %q\n got %+v\nwant %+v", in, got, want)
		}
	})
}

// hasNonCanonicalName reports names that only encoding/json's case-insensitive
// fallback, or its last-duplicate-wins rule, would bind.
func hasNonCanonicalName(in string) bool {
	names := []string{"name", "count", "big", "ratio", "ok", "tags", "nested"}
	seen := map[string]bool{}
	dec := json.NewDecoder(strings.NewReader(in))
	depth := 0
	expectName := false
	for {
		tok, err := dec.Token()
		if err != nil {
			return false
		}
		switch t := tok.(type) {
		case json.Delim:
			switch t {
			case '{':
				depth++
				expectName = true
			case '}':
				depth--
				expectName = depth > 0
			default:
				expectName = false
			}
		case string:
			if expectName {
				if depth == 1 {
					if seen[t] {
						return true
					}
					seen[t] = true
				}
				for _, n := range names {
					if t != n && strings.EqualFold(t, n) {
						return true
					}
				}
				expectName = false
				continue
			}
			expectName = depth > 0
		default:
			expectName = depth > 0
		}
	}
}

func TestIsBlank(t *testing.T) {
	for _, in := range []string{"", " ", "\t\r\n "} {
		if !IsBlank([]byte(in)) {
			t.Fatalf("want blank: %q", in)
		}
	}
	for _, in := range []string{"{}", " null ", "0"} {
		if IsBlank([]byte(in)) {
			t.Fatalf("want non-blank: %q", in)
		}
	}
}
