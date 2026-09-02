package jsonbind

import "testing"

// The three collection parsers carry the element type as a type parameter of
// their own, which no Go before 1.27 let a method declare. Each test pins that
// the deprecated function and the method decode the same document to the same
// value and annotate the same failure the same way.

func parserOver(data string) *Parser {
	var p Parser
	p.Reset([]byte(data))
	return &p
}

func sameError(t *testing.T, what string, fromFunc, fromMethod error) {
	t.Helper()
	if fromFunc == nil || fromMethod == nil {
		t.Fatalf("%s: function error %v, method error %v; both must fail", what, fromFunc, fromMethod)
	}
	if fromFunc.Error() != fromMethod.Error() {
		t.Fatalf("%s: errors differ:\n%s\n%s", what, fromFunc, fromMethod)
	}
}

func TestParseSliceFunctionAndMethodAgree(t *testing.T) {
	fromFunc, err := ParseSlice(parserOver(`[1,2,3]`), "n", "invalid int", (*Parser).Int)
	if err != nil {
		t.Fatalf("ParseSlice function: %v", err)
	}
	fromMethod, err := parserOver(`[1,2,3]`).ParseSlice("n", "invalid int", (*Parser).Int)
	if err != nil {
		t.Fatalf("ParseSlice method: %v", err)
	}
	if len(fromFunc) != 3 || len(fromMethod) != 3 {
		t.Fatalf("lengths differ: %d and %d", len(fromFunc), len(fromMethod))
	}
	for i := range fromFunc {
		if fromFunc[i] != fromMethod[i] {
			t.Fatalf("element %d differs: %d and %d", i, fromFunc[i], fromMethod[i])
		}
	}
	_, errFunc := ParseSlice(parserOver(`[1,"x"]`), "n", "invalid int", (*Parser).Int)
	_, errMethod := parserOver(`[1,"x"]`).ParseSlice("n", "invalid int", (*Parser).Int)
	sameError(t, "ParseSlice", errFunc, errMethod)
}

func TestParseArrayFunctionAndMethodAgree(t *testing.T) {
	var fromFunc, fromMethod [3]int
	if err := ParseArray(parserOver(`[1,2]`), "n", "invalid int", fromFunc[:], (*Parser).Int); err != nil {
		t.Fatalf("ParseArray function: %v", err)
	}
	if err := parserOver(`[1,2]`).ParseArray("n", "invalid int", fromMethod[:], (*Parser).Int); err != nil {
		t.Fatalf("ParseArray method: %v", err)
	}
	if fromFunc != fromMethod || fromFunc != [3]int{1, 2, 0} {
		t.Fatalf("arrays differ: %v and %v", fromFunc, fromMethod)
	}
	errFunc := ParseArray(parserOver(`[1,2,3,4]`), "n", "invalid int", fromFunc[:], (*Parser).Int)
	errMethod := parserOver(`[1,2,3,4]`).ParseArray("n", "invalid int", fromMethod[:], (*Parser).Int)
	sameError(t, "ParseArray", errFunc, errMethod)
}

func TestParseMapFunctionAndMethodAgree(t *testing.T) {
	fromFunc, err := ParseMap(parserOver(`{"a":1,"b":2}`), "n", "invalid int", (*Parser).Int)
	if err != nil {
		t.Fatalf("ParseMap function: %v", err)
	}
	fromMethod, err := parserOver(`{"a":1,"b":2}`).ParseMap("n", "invalid int", (*Parser).Int)
	if err != nil {
		t.Fatalf("ParseMap method: %v", err)
	}
	if len(fromFunc) != 2 || len(fromMethod) != 2 {
		t.Fatalf("sizes differ: %d and %d", len(fromFunc), len(fromMethod))
	}
	for k, v := range fromFunc {
		if fromMethod[k] != v {
			t.Fatalf("member %q differs: %d and %d", k, v, fromMethod[k])
		}
	}
	_, errFunc := ParseMap(parserOver(`{"a":"x"}`), "n", "invalid int", (*Parser).Int)
	_, errMethod := parserOver(`{"a":"x"}`).ParseMap("n", "invalid int", (*Parser).Int)
	sameError(t, "ParseMap", errFunc, errMethod)
}
