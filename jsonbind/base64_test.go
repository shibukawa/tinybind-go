package jsonbind_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/shibukawa/tinybind-go/jsonbind"
)

// The reason base64 was picked: another decoder reads what this one writes.
func TestAppendBase64MatchesEncodingJSON(t *testing.T) {
	for _, v := range [][]byte{nil, {}, {0}, {0xde, 0xad, 0xbe, 0xef}, bytes.Repeat([]byte{1, 2, 3}, 400)} {
		got := string(jsonbind.AppendBase64(nil, v))
		want, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		// A nil slice is the one divergence: encoding/json writes null, and
		// this codec writes "" for the same reason it writes [] for a nil
		// slice of anything else.
		if v == nil {
			if got != `""` {
				t.Fatalf("nil wrote %s", got)
			}
			continue
		}
		if got != string(want) {
			t.Fatalf("%v: wrote %s, encoding/json wrote %s", v, got, want)
		}
	}
}

func TestParseBase64(t *testing.T) {
	for _, tc := range []struct {
		name string
		doc  string
		want []byte
		err  bool
	}{
		{name: "value", doc: `"3q2+7w=="`, want: []byte{0xde, 0xad, 0xbe, 0xef}},
		{name: "empty", doc: `""`, want: []byte{}},
		{name: "null is nil", doc: `null`, want: nil},
		// \u0033 is '3', so this is the same payload written the slow way --
		// legal JSON, and the one path that goes through unescape scratch.
		{name: "escaped payload", doc: `"\u0033q2+7w=="`, want: []byte{0xde, 0xad, 0xbe, 0xef}},
		{name: "not base64", doc: `"not base64!!"`, err: true},
		{name: "not a string", doc: `[1,2]`, err: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var p jsonbind.Parser
			p.Reset([]byte(tc.doc))
			got, err := jsonbind.ParseBase64(&p, "blob")
			if tc.err {
				if err == nil {
					t.Fatalf("accepted %s as %v", tc.doc, got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, tc.want) || (got == nil) != (tc.want == nil) {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

// The fixed-length destination keeps ParseArray's two-ended contract, and the
// padding case matters: DecodedLen overshoots, so a payload it calls too long
// may still fit.
func TestParseBase64Into(t *testing.T) {
	for _, tc := range []struct {
		name  string
		doc   string
		start [4]byte
		want  [4]byte
		err   error
	}{
		{name: "exact", doc: `"AQIDBA=="`, want: [4]byte{1, 2, 3, 4}},
		{name: "short zeroes the tail", doc: `"AQ=="`, start: [4]byte{9, 9, 9, 9}, want: [4]byte{1, 0, 0, 0}},
		{name: "padding does not read as too long", doc: `"AQID"`, want: [4]byte{1, 2, 3, 0}},
		{name: "null leaves it alone", doc: `null`, start: [4]byte{9, 9, 9, 9}, want: [4]byte{9, 9, 9, 9}},
		{name: "too long", doc: `"AQIDBAU="`, err: jsonbind.ErrArrayTooLong},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var p jsonbind.Parser
			p.Reset([]byte(tc.doc))
			got := tc.start
			err := jsonbind.ParseBase64Into(&p, "tag", got[:])
			if tc.err != nil {
				if !errors.Is(err, tc.err) {
					t.Fatalf("err %v, want %v", err, tc.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}
