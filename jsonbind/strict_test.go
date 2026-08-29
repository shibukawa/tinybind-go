package jsonbind_test

import (
	"encoding/json"
	"testing"

	"github.com/shibukawa/tinybind-go/jsonbind"
)

// The parser's contract is encoding/json's grammar, stated in its own comments
// — and a lenient reader here is not a convenience but a differential: the
// document one parser accepts and another rejects is the classic smuggling
// seam, and a rest map's RawValue re-emits whatever this parser admitted to
// the next parser downstream as JSON.
//
// These pin the rejections that were once accepted. \q decoded to a literal q,
// a truncated \u to U+FFFD, and +1, .5, 01, 1. all decoded as numbers.
var strictCorpus = []struct {
	name string
	in   string
}{
	{"invalid escape", `"\q"`},
	{"hex escape", `"\x41"`},
	{"uppercase U escape", `"\U0041"`},
	{"truncated unicode escape", `"\u12"`},
	{"non-hex unicode escape", `"\u123z"`},
	{"quote inside unicode escape", `"\u12"3"`},
	{"escape at end of input", `"a\`},
	{"plus-signed number", `+1`},
	{"bare fraction", `.5`},
	{"leading zero", `01`},
	{"negative leading zero", `-01`},
	{"trailing point", `1.`},
	{"empty exponent", `1e`},
	{"signed empty exponent", `1e+`},
	{"bare minus", `-`},
	{"double minus", `--1`},
	{"two points", `1.2.3`},
	{"two exponents", `1e1e1`},
	{"hex number", `0x10`},
	{"invalid escape in key", `{"\q":1}`},
	{"invalid escape in skipped member", `{"junk":"\q","keep":1}`},
	{"invalid number in skipped member", `{"junk":+1,"keep":1}`},
	{"leading zero in array element", `[01]`},
}

func TestParserRejectsWhatEncodingJSONRejects(t *testing.T) {
	for _, tc := range strictCorpus {
		var std any
		if err := json.Unmarshal([]byte(tc.in), &std); err == nil {
			t.Fatalf("%s: corpus entry %q is valid JSON; it does not belong here", tc.name, tc.in)
		}
		if _, err := jsonbind.DecodeJSONAny([]byte(tc.in)); err == nil {
			t.Errorf("%s: DecodeJSONAny accepted %q, encoding/json rejects it", tc.name, tc.in)
		}
	}
}

// SkipValue walks the same spans through stringSpan and numberSpan, so a
// malformed member must fail the walk even when no field wants it — otherwise
// RawValue hands the bytes onward as JSON.
func TestSkipValueRejectsWhatEncodingJSONRejects(t *testing.T) {
	for _, tc := range strictCorpus {
		p := jsonbind.NewParser([]byte(tc.in))
		err := p.SkipValue()
		if err == nil {
			err = p.End()
		}
		if err == nil {
			t.Errorf("%s: SkipValue walked %q without error, encoding/json rejects it", tc.name, tc.in)
		}
	}
}

// The still-valid side of the same grammar corners, so the strictness cannot
// creep past what encoding/json actually rejects.
func TestParserAcceptsWhatEncodingJSONAccepts(t *testing.T) {
	for _, in := range []string{
		`"ካ"`, `"😀"`, `"\ud800"`, `"\/"`, "\"\u007f\"",
		`0`, `-0`, `0.5`, `-0.5e+10`, `20`, `1e2`, `1E-2`, `123456789012345678901234567890`,
		`{"aA":[0.1,"\n"]}`,
	} {
		var std any
		if err := json.Unmarshal([]byte(in), &std); err != nil {
			t.Fatalf("corpus entry %q is invalid JSON; it does not belong here: %v", in, err)
		}
		if _, err := jsonbind.DecodeJSONAny([]byte(in)); err != nil {
			t.Errorf("DecodeJSONAny rejected %q, encoding/json accepts it: %v", in, err)
		}
	}
}

// FuzzParserRejectionMatchesEncodingJSON holds the whole accept/reject
// boundary against encoding/json, not just the corners the corpus names. The
// fuzz beside it in parser_test.go drives the accepted side's values; this one
// exists for the rejected side, which is where the smuggling lived. String
// results are compared exactly; other shapes only for acceptance, since both
// sides decode numbers to float64 anyway.
func FuzzParserRejectionMatchesEncodingJSON(f *testing.F) {
	for _, tc := range strictCorpus {
		f.Add(tc.in)
	}
	f.Add(`{"a":[1,2.5,"x",null,true]}`)
	f.Add(`"😀"`)
	f.Fuzz(func(t *testing.T, in string) {
		var std any
		stdErr := json.Unmarshal([]byte(in), &std)
		ours, oursErr := jsonbind.DecodeJSONAny([]byte(in))
		if (stdErr == nil) != (oursErr == nil) {
			t.Fatalf("%q: encoding/json err=%v, DecodeJSONAny err=%v", in, stdErr, oursErr)
		}
		if stdErr == nil {
			if s, ok := std.(string); ok {
				if got, ok := ours.(string); !ok || got != s {
					t.Fatalf("%q: encoding/json %q, DecodeJSONAny %v", in, s, ours)
				}
			}
		}
	})
}
