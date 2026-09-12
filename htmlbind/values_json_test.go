package htmlbind

import (
	"strings"
	"testing"
)

func TestAppendJSONStringEscapesScriptContent(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{`"`, `"\""`},
		{`\`, `"\\"`},
		{"<script>&", `"\u003cscript\u003e\u0026"`},
		{"\x00\b\f\n\r\t", `"\u0000\b\f\n\r\t"`},
		{"\u2028\u2029", `"\u2028\u2029"`},
		{"\xff", `"�"`},
	} {
		if got := string(AppendJSONString(nil, tc.in)); got != tc.want {
			t.Errorf("AppendJSONString(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// A named string type and a named byte-slice type reach the same escaper the
// plain kinds do, which is what lets a caller pass TrustedHTML or a rendered
// fragment without converting first.
func TestAppendJSONStringTakesNamedTypes(t *testing.T) {
	type namedBytes []byte
	const input = `<b>café</b> & "more"`
	want := JSONString(input)
	if got := string(AppendJSONString(nil, TrustedHTML(input))); got != want {
		t.Errorf("AppendJSONString(TrustedHTML) = %q, want %q", got, want)
	}
	if got := string(AppendJSONString(nil, namedBytes(input))); got != want {
		t.Errorf("AppendJSONString(namedBytes) = %q, want %q", got, want)
	}
}

// Appending extends what dst already holds rather than replacing it, which is
// the whole point of the entry: a caller assembles a record around the call.
func TestAppendJSONStringExtendsDst(t *testing.T) {
	dst := AppendJSONString([]byte(`{"id":`), "b-1")
	dst = append(dst, `,"html":`...)
	dst = AppendJSONString(dst, []byte("<p>hi</p>"))
	if got, want := string(append(dst, '}')), `{"id":"b-1","html":"\u003cp\u003ehi\u003c/p\u003e"}`; got != want {
		t.Errorf("assembled record = %q, want %q", got, want)
	}
}

// A short dst is grown rather than overrun, so a nil scratch stays legal.
func TestAppendJSONStringGrowsAShortDst(t *testing.T) {
	for _, dst := range [][]byte{nil, make([]byte, 0, 1), make([]byte, 3, 4)} {
		prefix := string(dst)
		got := string(AppendJSONString(dst, strings.Repeat("<x>", 64)))
		if want := prefix + JSONString(strings.Repeat("<x>", 64)); got != want {
			t.Errorf("AppendJSONString(cap %d) = %q, want %q", cap(dst), got, want)
		}
	}
}

// allocSink escapes so that the probe below allocates on any toolchain that
// counts allocations at all.
var allocSink []byte

// allocationsAreCounted reports whether this toolchain counts allocations at
// all. TinyGo's AllocsPerRun returns zero for an allocation that plainly
// escapes, so a zero from it means nothing was measured rather than nothing was
// allocated, and asserting on it would leave the tests below passing for no
// reason.
//
// Callers log and return rather than calling t.Skip, because on TinyGo's wasm
// targets SkipNow does not call runtime.Goexit: a test skipped there runs on to
// the assertion it was skipping and is reported as a failure.
func allocationsAreCounted() bool {
	return testing.AllocsPerRun(10, func() { allocSink = make([]byte, 4096) }) > 0
}

const uncounted = "this toolchain does not count allocations; nothing asserted"

// The entry exists so a caller assembling into a reused buffer stops paying per
// field. A []byte fragment appended into a buffer with room costs nothing;
// JSONString on the same fragment costs three allocations.
func TestAppendJSONStringAppendsWithoutAllocating(t *testing.T) {
	if !allocationsAreCounted() {
		t.Log(uncounted)
		return
	}
	fragment := []byte(strings.Repeat(`<td class="cell">value</td>`, 32))
	scratch := make([]byte, 0, 8192)
	if allocs := testing.AllocsPerRun(200, func() {
		_ = AppendJSONString(scratch[:0], fragment)
	}); allocs != 0 {
		t.Errorf("AppendJSONString into a pre-grown buffer allocated %v times, want 0", allocs)
	}
	if allocs := testing.AllocsPerRun(200, func() {
		_ = append(scratch[:0], JSONString(string(fragment))...)
	}); allocs == 0 {
		t.Error("JSONString(string(fragment)) allocated nothing; the corpus no longer measures what the entry removes")
	}
}

// Content.AppendJSON is the module's own record writer and holds its fragment
// as bytes, so it is the first caller the export was meant to relieve.
func TestContentAppendJSONEscapesItsFragmentWithoutConverting(t *testing.T) {
	content := Content{BoundaryID: "tb-3", HTML: []byte(strings.Repeat("<span>x</span>", 32))}
	want := `{"id":` + JSONString(content.BoundaryID) + `,"html":` + JSONString(string(content.HTML)) + `}`
	if got := string(content.AppendJSON(nil)); got != want {
		t.Errorf("Content.AppendJSON = %q, want %q", got, want)
	}
	if !allocationsAreCounted() {
		t.Log(uncounted)
		return
	}
	scratch := make([]byte, 0, 8192)
	if allocs := testing.AllocsPerRun(200, func() {
		_ = content.AppendJSON(scratch[:0])
	}); allocs != 0 {
		t.Errorf("Content.AppendJSON into a pre-grown buffer allocated %v times, want 0", allocs)
	}
}
