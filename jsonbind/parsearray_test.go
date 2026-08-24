package jsonbind_test

import (
	"errors"
	"testing"

	"github.com/shibukawa/tinybind-go/jsonbind"
)

// ParseArray is the one runtime piece a fixed-length field rests on, so its
// four answers are pinned here rather than only through generated code.
func TestParseArray(t *testing.T) {
	for _, tc := range []struct {
		name  string
		doc   string
		start [3]int
		want  [3]int
		err   error
	}{
		{name: "exact", doc: `[1,2,3]`, want: [3]int{1, 2, 3}},
		{name: "short fills what arrived", doc: `[7]`, want: [3]int{7, 0, 0}},
		{name: "short zeroes what was there", doc: `[7]`, start: [3]int{9, 9, 9}, want: [3]int{7, 0, 0}},
		{name: "empty", doc: `[]`, start: [3]int{9, 9, 9}, want: [3]int{}},
		{name: "null leaves it alone", doc: `null`, start: [3]int{9, 9, 9}, want: [3]int{9, 9, 9}},
		{name: "long is refused", doc: `[1,2,3,4]`, want: [3]int{}, err: jsonbind.ErrArrayTooLong},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var p jsonbind.Parser
			p.Reset([]byte(tc.doc))
			got := tc.start
			err := jsonbind.ParseArray(&p, "cells", "invalid int", got[:], (*jsonbind.Parser).Int)
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

// The member name and the limit are both in the message, which is what a
// caller reads when the field is one of twenty.
func TestParseArrayNamesTheFieldAndTheLimit(t *testing.T) {
	var p jsonbind.Parser
	p.Reset([]byte(`[1,2,3,4]`))
	var dst [3]int
	err := jsonbind.ParseArray(&p, "cells", "invalid int", dst[:], (*jsonbind.Parser).Int)
	je, ok := jsonbind.AsError(err)
	if !ok {
		t.Fatalf("not a jsonbind error: %v", err)
	}
	if je.Field != "cells" || je.Message != "expected at most 3 elements" {
		t.Fatalf("field %q message %q", je.Field, je.Message)
	}
}
