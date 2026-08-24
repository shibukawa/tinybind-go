package generator_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

// fixedArraySource declares a fixed-length array of every shape a field can
// take -- a scalar element, a string element, a struct element, and a length
// written as a constant -- beside a slice, so one fixture covers both kinds
// through the binder, the encoder and the CBOR codecs at once.
const fixedArraySource = `package main

import (
	"net/http"

	"github.com/shibukawa/tinybind-go"
)

const seatCount = 3

type Mark struct {
	X int ` + "`json:\"x\"`" + `
	Y int ` + "`json:\"y\"`" + `
}

type Board struct {
	Cells [4]int           ` + "`payload:\"cells\"`" + `
	Names [2]string        ` + "`payload:\"names\"`" + `
	Marks [2]Mark          ` + "`payload:\"marks\"`" + `
	Seats [seatCount]uint16 ` + "`payload:\"seats\"`" + `
	Tags  []string         ` + "`payload:\"tags\"`" + `
}

type Result struct {
	Cells [4]int           ` + "`json:\"cells\"`" + `
	Names [2]string        ` + "`json:\"names\"`" + `
	Marks [2]Mark          ` + "`json:\"marks\"`" + `
	Seats [seatCount]uint16 ` + "`json:\"seats\"`" + `
	Tags  []string         ` + "`json:\"tags\"`" + `
}

func Handler(w http.ResponseWriter, r *http.Request) {
	in, err := httpbind.Bind[Board](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	_ = httpbind.Write[Result](w, r, Result(in))
}

func main() {
	http.HandleFunc("POST /boards", Handler)
}
`

func emitFixedArray(t *testing.T, src string, opts generator.Options) (string, string) {
	t.Helper()
	dir := t.TempDir()
	writeTempModule(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	plan, err := generator.AnalyzePackageWithOptions(dir, opts)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	code, err := generator.Emit(plan)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	return dir, string(code)
}

// The regression this whole change exists for: a fixed-length field used to be
// planned as a slice, so the decoder assigned a []int to a [4]int inside a file
// headed DO NOT EDIT.
func TestFixedArrayIsFilledInPlace(t *testing.T) {
	opts := generator.DefaultOptions()
	opts.EnableCBORHTTP = true
	_, source := emitFixedArray(t, fixedArraySource, opts)
	for _, want := range []string{
		`jsonbind.ParseArray(p, "cells", "invalid int", out.Cells[:], (*jsonbind.Parser).Int)`,
		`jsonbind.ParseArray(p, "marks", "", out.Marks[:], decodeMarkJSON)`,
		"out.Seats = [seatCount]uint16{}",
		"if n > len(out.Cells) {",
	} {
		if !strings.Contains(source, want) {
			t.Errorf("generated source is missing %q", want)
		}
	}
	// The slice assignment is what did not compile; a slice field still gets it.
	if strings.Contains(source, "out.Cells = v") || strings.Contains(source, "out.Cells = slice") {
		t.Error("a fixed-length field is still assigned a slice")
	}
	if !strings.Contains(source, `jsonbind.ParseSlice(p, "tags"`) {
		t.Error("a slice field no longer decodes through ParseSlice")
	}
}

// Encoding is the one direction the two kinds agree about, so it shares an arm
// rather than growing one.
func TestFixedArrayEncodesLikeASlice(t *testing.T) {
	_, source := emitFixedArray(t, fixedArraySource, generator.DefaultOptions())
	for _, want := range []string{"for i := range v.Cells {", "jsonbind.AppendInt(dst, int64(v.Cells[i]))"} {
		if !strings.Contains(source, want) {
			t.Errorf("generated encoder is missing %q", want)
		}
	}
}

// A fixed-length array of a named scalar is generated like every other
// collection of one; the refusal it used to get was lifted with them.
func TestFixedArrayOfNamedElementIsGenerated(t *testing.T) {
	src := strings.Replace(fixedArraySource, "Cells [4]int           ", "Cells [4]Level         ", 1)
	src = strings.Replace(src, "const seatCount = 3", "const seatCount = 3\n\ntype Level int", 1)
	dir := t.TempDir()
	writeTempModule(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	plan, err := generator.AnalyzePackageWithOptions(dir, generator.DefaultOptions())
	if err != nil {
		t.Fatalf("an array of a named scalar was refused: %v", err)
	}
	code, err := generator.Emit(plan)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(code), "out.Cells[:], func(p *jsonbind.Parser) (Level, error)") {
		t.Fatalf("the array does not fill with the declared type:\n%s", code)
	}
}

// omitzero compares against the zero value, and a Go array is comparable only
// when its element is. A struct element is refused with the reason rather than
// emitted as a comparison that would not compile.
func TestOmitZeroOnAnArrayOfStructsIsRefused(t *testing.T) {
	src := strings.Replace(fixedArraySource,
		"Marks [2]Mark          "+"`json:\"marks\"`",
		"Marks [2]Mark          "+"`json:\"marks,omitzero\"`", 1)
	dir := t.TempDir()
	writeTempModule(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	_, err := generator.AnalyzePackageWithOptions(dir, generator.DefaultOptions())
	if err == nil {
		t.Fatal("omitzero on an array of structs was accepted")
	}
	if !strings.Contains(err.Error(), "omitzero is not available on an array of Mark") {
		t.Fatalf("unexpected refusal: %v", err)
	}
}

// The end of the line: the generated code compiles, and a fixed-length field
// fills what arrived, zeroes what did not, and refuses what does not fit --
// through JSON and through CBOR alike.
func TestFixedArraysRoundTripOverHTTP(t *testing.T) {
	opts := generator.DefaultOptions()
	opts.EnableCBORHTTP = true
	dir, code := emitFixedArray(t, fixedArraySource, opts)
	if err := os.WriteFile(filepath.Join(dir, "tinybind_gen.go"), []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	probe := `package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shibukawa/tinygodriver/encoding/cbor"
)

func post(t *testing.T, payload string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/boards", strings.NewReader(payload))
	r.Header.Set("Content-Type", "application/json")
	Handler(w, r)
	return w
}

func decode(t *testing.T, w *httptest.ResponseRecorder) Result {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var got Result
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestAFullArrayRoundTrips(t *testing.T) {
	got := decode(t, post(t, ` + "`" + `{"cells":[1,2,3,4],"names":["a","b"],"marks":[{"x":1,"y":2},{"x":3,"y":4}],"seats":[7,8,9],"tags":["t"]}` + "`" + `))
	if got.Cells != [4]int{1, 2, 3, 4} {
		t.Fatalf("cells %v", got.Cells)
	}
	if got.Names != [2]string{"a", "b"} {
		t.Fatalf("names %v", got.Names)
	}
	if got.Marks != [2]Mark{{X: 1, Y: 2}, {X: 3, Y: 4}} {
		t.Fatalf("marks %v", got.Marks)
	}
	if got.Seats != [seatCount]uint16{7, 8, 9} {
		t.Fatalf("seats %v", got.Seats)
	}
	if len(got.Tags) != 1 || got.Tags[0] != "t" {
		t.Fatalf("tags %v", got.Tags)
	}
}

// The length is the type's statement, so a document need not restate it.
func TestAShortArrayLeavesTheTailZero(t *testing.T) {
	got := decode(t, post(t, ` + "`" + `{"cells":[7],"names":[],"marks":[{"x":5,"y":6}],"seats":[1,2]}` + "`" + `))
	if got.Cells != [4]int{7, 0, 0, 0} {
		t.Fatalf("cells %v", got.Cells)
	}
	if got.Names != [2]string{"", ""} {
		t.Fatalf("names %v", got.Names)
	}
	if got.Marks != [2]Mark{{X: 5, Y: 6}, {}} {
		t.Fatalf("marks %v", got.Marks)
	}
	if got.Seats != [seatCount]uint16{1, 2, 0} {
		t.Fatalf("seats %v", got.Seats)
	}
}

// Storing the first four and dropping the fifth is the silent loss the declared
// length exists to prevent.
func TestALongArrayIs400(t *testing.T) {
	if w := post(t, ` + "`" + `{"cells":[1,2,3,4,5]}` + "`" + `); w.Code != http.StatusBadRequest {
		t.Fatalf("status %d, body %s", w.Code, w.Body.String())
	}
	if w := post(t, ` + "`" + `{"marks":[{"x":1},{"x":2},{"x":3}]}` + "`" + `); w.Code != http.StatusBadRequest {
		t.Fatalf("status %d, body %s", w.Code, w.Body.String())
	}
}

// A JSON null leaves the destination alone, as it does for a slice.
func TestANullArrayLeavesTheFieldAlone(t *testing.T) {
	got := decode(t, post(t, ` + "`" + `{"cells":null}` + "`" + `))
	if got.Cells != [4]int{} {
		t.Fatalf("cells %v", got.Cells)
	}
}

// A member arriving twice decodes to the second array, not to the two overlaid.
func TestARepeatedMemberDoesNotOverlay(t *testing.T) {
	got := decode(t, post(t, ` + "`" + `{"cells":[1,2,3,4],"cells":[9]}` + "`" + `))
	if got.Cells != [4]int{9, 0, 0, 0} {
		t.Fatalf("cells %v", got.Cells)
	}
}

func cborBoard(cells []int64) []byte {
	in := cbor.AppendMapHeader(nil, 2)
	in = cbor.AppendText(in, "cells")
	in = cbor.AppendArrayHeader(in, len(cells))
	for _, c := range cells {
		in = cbor.AppendInt(in, c)
	}
	in = cbor.AppendText(in, "marks")
	in = cbor.AppendArrayHeader(in, 1)
	in = cbor.AppendMapHeader(in, 2)
	in = cbor.AppendText(in, "x")
	in = cbor.AppendInt(in, 1)
	in = cbor.AppendText(in, "y")
	in = cbor.AppendInt(in, 2)
	return in
}

func postCBOR(t *testing.T, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/boards", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/cbor")
	Handler(w, r)
	return w
}

func TestCBORFillsAndZeroesTheSameWay(t *testing.T) {
	got := decode(t, postCBOR(t, cborBoard([]int64{5, 6})))
	if got.Cells != [4]int{5, 6, 0, 0} {
		t.Fatalf("cells %v", got.Cells)
	}
	if got.Marks != [2]Mark{{X: 1, Y: 2}, {}} {
		t.Fatalf("marks %v", got.Marks)
	}
}

func TestCBORRefusesALongArray(t *testing.T) {
	if w := postCBOR(t, cborBoard([]int64{1, 2, 3, 4, 5})); w.Code != http.StatusBadRequest {
		t.Fatalf("status %d, body %s", w.Code, w.Body.String())
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "probe_test.go"), []byte(probe), 0o644); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "test", "./...")
	command.Dir = dir
	command.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("fixed-length arrays do not round-trip: %v\n%s\n%s", err, output, code)
	}
	// go test exits 0 on a module it found nothing to run in, which would make
	// this whole fixture a green test that proved nothing.
	if strings.Contains(string(output), "no test files") {
		t.Fatalf("the probe did not run: %s", output)
	}
}

// codecFixedArraySource asks for the standalone codecs -- the jsonbind document
// pair and both cborbind shapes -- rather than the HTTP mapping, since those
// are separate emitters reading the same plan.
const codecFixedArraySource = `package main

import (
	"github.com/shibukawa/tinybind-go/cborbind"
	"github.com/shibukawa/tinybind-go/jsonbind"
)

type Frame struct {
	Cells [4]int    ` + "`json:\"cells\"`" + `
	Names [2]string ` + "`json:\"names\"`" + `
}

var _ = jsonbind.GenerateCodec[Frame]()

var buf []byte

func main() {
	buf = cborbind.AppendCBORInMapTo(buf[:0], Frame{})
	if f, err := cborbind.DecodeCBORInMapFrom[Frame](buf); err == nil {
		_ = f
	}
	buf = cborbind.AppendCBORInArrayTo(buf[:0], Frame{})
	if f, err := cborbind.DecodeCBORInArrayFrom[Frame](buf); err == nil {
		_ = f
	}
}
`

// The standalone codecs carry a fixed-length field on the same terms, and the
// CBOR half reports the driver's limit error rather than handing up the nil one
// it has in scope at the count comparison.
func TestFixedArraysRoundTripThroughTheStandaloneCodecs(t *testing.T) {
	dir, code := emitFixedArray(t, codecFixedArraySource, generator.DefaultOptions())
	if !strings.Contains(code, "return out, cbor.ErrLimitExceeded") {
		t.Fatalf("the CBOR codec does not refuse a long array:\n%s", code)
	}
	if err := os.WriteFile(filepath.Join(dir, "tinybind_gen.go"), []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	probe := `package main

import (
	"errors"
	"testing"

	"github.com/shibukawa/tinybind-go/cborbind"
	"github.com/shibukawa/tinybind-go/jsonbind"
	"github.com/shibukawa/tinygodriver/encoding/cbor"
)

func TestJSONCodecFillsAndZeroes(t *testing.T) {
	got, err := jsonbind.DecodeJSONBytes[Frame]([]byte(` + "`" + `{"cells":[1,2],"names":["a","b"]}` + "`" + `))
	if err != nil {
		t.Fatal(err)
	}
	if got.Cells != [4]int{1, 2, 0, 0} || got.Names != [2]string{"a", "b"} {
		t.Fatalf("got %+v", got)
	}
	if out := string(got.AppendJSONTo(nil)); out != ` + "`" + `{"cells":[1,2,0,0],"names":["a","b"]}` + "`" + ` {
		t.Fatalf("encoded %s", out)
	}
}

func TestJSONCodecRefusesALongArray(t *testing.T) {
	_, err := jsonbind.DecodeJSONBytes[Frame]([]byte(` + "`" + `{"cells":[1,2,3,4,5]}` + "`" + `))
	if !errors.Is(err, jsonbind.ErrArrayTooLong) {
		t.Fatalf("err %v", err)
	}
}

func TestCBORCodecRefusesALongArray(t *testing.T) {
	in := cbor.AppendMapHeader(nil, 1)
	in = cbor.AppendText(in, "cells")
	in = cbor.AppendArrayHeader(in, 5)
	for i := 0; i < 5; i++ {
		in = cbor.AppendInt(in, int64(i))
	}
	if _, err := cborbind.DecodeCBORInMapFrom[Frame](in); !errors.Is(err, cbor.ErrLimitExceeded) {
		t.Fatalf("err %v", err)
	}
}

func TestCBORCodecRoundTrips(t *testing.T) {
	in := cborbind.AppendCBORInArrayTo(nil, Frame{Cells: [4]int{1, 2, 3, 4}, Names: [2]string{"a", "b"}})
	got, err := cborbind.DecodeCBORInArrayFrom[Frame](in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Cells != [4]int{1, 2, 3, 4} || got.Names != [2]string{"a", "b"} {
		t.Fatalf("got %+v", got)
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "probe_test.go"), []byte(probe), 0o644); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "test", "./...")
	command.Dir = dir
	command.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("the standalone codecs do not carry a fixed-length array: %v\n%s\n%s", err, output, code)
	}
	if strings.Contains(string(output), "no test files") {
		t.Fatalf("the probe did not run: %s", output)
	}
}
