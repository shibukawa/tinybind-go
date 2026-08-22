package generator_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

// sizedIntegerSource declares one field of every admitted width, a slice and a
// map of a narrow one, and a named type over a sized integer, so a single
// fixture reaches the encoder, the document decoder, the query binder and the
// CBOR codec at once.
const sizedIntegerSource = `package main

import (
	"net/http"

	"github.com/shibukawa/tinybind-go"
)

type Level uint16

type Reading struct {
	Tick    uint32   ` + "`payload:\"tick\"`" + `
	Seq     uint16   ` + "`payload:\"seq\"`" + `
	Btn     uint8    ` + "`payload:\"btn\"`" + `
	AxisX   int16    ` + "`payload:\"axisX\"`" + `
	AxisY   int8     ` + "`payload:\"axisY\"`" + `
	Aim     int32    ` + "`payload:\"aim\"`" + `
	Total   uint64   ` + "`payload:\"total\"`" + `
	Count   uint     ` + "`payload:\"count\"`" + `
	Lane    Level    ` + "`payload:\"lane\"`" + `
	Samples []uint32 ` + "`payload:\"samples\"`" + `
	Limit   uint16   ` + "`query:\"limit\"`" + `
}

type Result struct {
	Tick    uint32   ` + "`json:\"tick\"`" + `
	Seq     uint16   ` + "`json:\"seq\"`" + `
	Btn     uint8    ` + "`json:\"btn\"`" + `
	AxisX   int16    ` + "`json:\"axisX\"`" + `
	AxisY   int8     ` + "`json:\"axisY\"`" + `
	Aim     int32    ` + "`json:\"aim\"`" + `
	Total   uint64   ` + "`json:\"total\"`" + `
	Count   uint     ` + "`json:\"count\"`" + `
	Lane    Level    ` + "`json:\"lane\"`" + `
	Samples []uint32 ` + "`json:\"samples\"`" + `
	Limit   uint16   ` + "`json:\"limit\"`" + `
}

func Handler(w http.ResponseWriter, r *http.Request) {
	in, err := httpbind.Bind[Reading](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	_ = httpbind.Write[Result](w, r, Result(in))
}

func main() {
	http.HandleFunc("POST /readings", Handler)
}
`

// emitSized writes the fixture into a temp module and returns the directory
// and the generated source, so a test can assert over either.
func emitSized(t *testing.T, src string, opts generator.Options) (string, string) {
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

// The whole point: a struct of narrow fields is planned rather than refused,
// and each width reaches the encoder as itself.
func TestSizedIntegersAreEmitted(t *testing.T) {
	_, source := emitSized(t, sizedIntegerSource, generator.DefaultOptions())
	for _, want := range []string{
		"jsonbind.AppendUint(dst, uint64(v.Tick))",
		"jsonbind.AppendInt(dst, int64(v.AxisX))",
		"jsonbind.AppendUint(dst, uint64(v.Lane))",
		"httpbind.ParseUintBits(",
	} {
		if !strings.Contains(source, want) {
			t.Errorf("generated source is missing %q", want)
		}
	}
}

// The bound is a literal the emitter knows, so the runtime never grows a
// method per width.
func TestSizedIntegerDecodeChecksTheWidth(t *testing.T) {
	_, source := emitSized(t, sizedIntegerSource, generator.DefaultOptions())
	for _, want := range []string{
		"v, err := p.Uint64()",
		"if v < 0 || v > 4294967295 {",
		"if v < -32768 || v > 32767 {",
		"jsonbind.ErrIntegerRange",
	} {
		if !strings.Contains(source, want) {
			t.Errorf("generated source is missing the width check %q", want)
		}
	}
	// uint64 and uint need no comparison; the parse itself is the refusal.
	if strings.Contains(source, "18446744073709551615") {
		t.Error("uint64 should need no emitted upper bound")
	}
}

// A slice element with no parser method of its own is read through an emitted
// closure, which is where its range check lives.
func TestSizedIntegerSliceReadsThroughAClosure(t *testing.T) {
	_, source := emitSized(t, sizedIntegerSource, generator.DefaultOptions())
	want := "func(p *jsonbind.Parser) (uint32, error)"
	if !strings.Contains(source, want) {
		t.Fatalf("generated source is missing the element closure %q\n%s", want, source)
	}
	// gofmt breaks the emitted closure over lines, so the conversion is
	// asserted on its own rather than as part of a one-line body.
	if !strings.Contains(source, "return uint32(v), nil") {
		t.Error("the element closure does not convert to the element width")
	}
	if !strings.Contains(source, "return 0, jsonbind.ErrIntegerRange") {
		t.Error("the element closure does not range-check the element")
	}
}

// The binder enforces a range, so the document has to state it or it describes
// a different API than the one running.
func TestSizedIntegerOpenAPIStatesTheRange(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(sizedIntegerSource), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	doc, err := generator.BuildOpenAPI(dir)
	if err != nil {
		t.Fatalf("openapi: %v", err)
	}
	encoded, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, want := range []string{`"format":"int32"`, `"maximum":65535`, `"minimum":-32768`} {
		if !strings.Contains(text, want) {
			t.Errorf("openapi document is missing %q", want)
		}
	}
	if strings.Contains(text, `"tick":{"type":"string"`) {
		t.Error("a sized integer is documented as a string")
	}
}

// The end of the line: the generated code compiles, binds, round-trips every
// width through JSON and through CBOR, and refuses a value the width cannot
// hold rather than wrapping it.
func TestSizedIntegersRoundTripOverHTTP(t *testing.T) {
	opts := generator.DefaultOptions()
	opts.EnableCBORHTTP = true
	dir, code := emitSized(t, sizedIntegerSource, opts)
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

const body = ` + "`" + `{"tick":4294967295,"seq":65535,"btn":255,"axisX":-32768,"axisY":-128,` +
		`"aim":-2147483648,"total":18446744073709551615,"count":7,"lane":42,"samples":[1,4294967295]}` + "`" + `

func post(t *testing.T, payload, query string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/readings?limit=900"+query, strings.NewReader(payload))
	r.Header.Set("Content-Type", "application/json")
	Handler(w, r)
	return w
}

func TestEveryWidthRoundTrips(t *testing.T) {
	w := post(t, body, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var got Result
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	want := Result{
		Tick: 4294967295, Seq: 65535, Btn: 255, AxisX: -32768, AxisY: -128,
		Aim: -2147483648, Total: 18446744073709551615, Count: 7, Lane: 42,
		Samples: []uint32{1, 4294967295}, Limit: 900,
	}
	if got.Tick != want.Tick || got.Seq != want.Seq || got.Btn != want.Btn ||
		got.AxisX != want.AxisX || got.AxisY != want.AxisY || got.Aim != want.Aim ||
		got.Count != want.Count || got.Lane != want.Lane || got.Limit != want.Limit {
		t.Fatalf("got %+v want %+v", got, want)
	}
	if got.Total != want.Total {
		t.Fatalf("uint64 above the signed range lost precision: got %d want %d", got.Total, want.Total)
	}
	if len(got.Samples) != 2 || got.Samples[1] != 4294967295 {
		t.Fatalf("slice element lost its width: %v", got.Samples)
	}
}

func TestOverWideMemberIsRejected(t *testing.T) {
	over := strings.Replace(body, ` + "`" + `"tick":4294967295` + "`" + `, ` + "`" + `"tick":4294967296` + "`" + `, 1)
	if w := post(t, over, ""); w.Code != http.StatusBadRequest {
		t.Fatalf("status %d, body %s", w.Code, w.Body.String())
	}
}

func TestOverWideSliceElementIsRejected(t *testing.T) {
	over := strings.Replace(body, ` + "`" + `[1,4294967295]` + "`" + `, ` + "`" + `[1,4294967296]` + "`" + `, 1)
	if w := post(t, over, ""); w.Code != http.StatusBadRequest {
		t.Fatalf("status %d, body %s", w.Code, w.Body.String())
	}
}

func TestNegativeIntoUnsignedIsRejected(t *testing.T) {
	neg := strings.Replace(body, ` + "`" + `"btn":255` + "`" + `, ` + "`" + `"btn":-1` + "`" + `, 1)
	if w := post(t, neg, ""); w.Code != http.StatusBadRequest {
		t.Fatalf("status %d, body %s", w.Code, w.Body.String())
	}
}

func TestOverWideQueryValueIs400(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/readings?limit=65536", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	Handler(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d, body %s", w.Code, w.Body.String())
	}
}

func TestCBORCarriesTheWidths(t *testing.T) {
	in := cbor.AppendMapHeader(nil, 10)
	in = cbor.AppendText(in, "tick")
	in = cbor.AppendUint(in, 4294967295)
	in = cbor.AppendText(in, "seq")
	in = cbor.AppendUint(in, 65535)
	in = cbor.AppendText(in, "btn")
	in = cbor.AppendUint(in, 255)
	in = cbor.AppendText(in, "axisX")
	in = cbor.AppendInt(in, -32768)
	in = cbor.AppendText(in, "axisY")
	in = cbor.AppendInt(in, -128)
	in = cbor.AppendText(in, "aim")
	in = cbor.AppendInt(in, -2147483648)
	in = cbor.AppendText(in, "total")
	in = cbor.AppendUint(in, 18446744073709551615)
	in = cbor.AppendText(in, "count")
	in = cbor.AppendUint(in, 7)
	in = cbor.AppendText(in, "lane")
	in = cbor.AppendUint(in, 42)
	in = cbor.AppendText(in, "samples")
	in = cbor.AppendArrayHeader(in, 2)
	in = cbor.AppendUint(in, 1)
	in = cbor.AppendUint(in, 4294967295)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/readings?limit=900", bytes.NewReader(in))
	r.Header.Set("Content-Type", "application/cbor")
	r.Header.Set("Accept", "application/cbor")
	Handler(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	rd := cbor.ReaderOver(w.Body.Bytes(), cbor.DecoderOptions{})
	pairs, indef, err := rd.ReadMapHeader()
	if err != nil || indef || pairs != 11 {
		t.Fatalf("map header %d indef=%v err=%v", pairs, indef, err)
	}
}

func TestCBOROverWideMemberIsRejected(t *testing.T) {
	in := cbor.AppendMapHeader(nil, 1)
	in = cbor.AppendText(in, "seq")
	in = cbor.AppendUint(in, 65536)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/readings?limit=1", bytes.NewReader(in))
	r.Header.Set("Content-Type", "application/cbor")
	Handler(w, r)
	if w.Code != http.StatusBadRequest {
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
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generated code does not carry the widths: %v\n%s\n%s", err, output, code)
	}
}
