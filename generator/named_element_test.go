package generator_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

// namedElementSource puts a named scalar in every collection shape, at a width
// with its own parser method and at one without, so both element-reader paths
// are covered.
const namedElementSource = `package main

import (
	"net/http"

	"github.com/shibukawa/tinybind-go"
)

type Mark uint
type Name string
type Level uint16

type Board struct {
	Marks  []Mark          ` + "`payload:\"marks\"`" + `
	Names  []Name          ` + "`payload:\"names\"`" + `
	Slots  [3]Level        ` + "`payload:\"slots\"`" + `
	ByName map[string]Mark ` + "`payload:\"byName\"`" + `
}

type Result struct {
	Marks  []Mark          ` + "`json:\"marks\"`" + `
	Names  []Name          ` + "`json:\"names\"`" + `
	Slots  [3]Level        ` + "`json:\"slots\"`" + `
	ByName map[string]Mark ` + "`json:\"byName\"`" + `
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

// The refusal this lifts: the bulk decoder answers a concrete []uint, which Go
// will not assign to a []Mark. The closure is the conversion, written once as a
// shape rather than once per element kind.
func TestNamedElementReadsThroughAClosure(t *testing.T) {
	opts := generator.DefaultOptions()
	opts.EnableCBORHTTP = true
	_, source := emitFixedArray(t, namedElementSource, opts)
	for _, want := range []string{
		"func(p *jsonbind.Parser) (Mark, error)",
		"func(p *jsonbind.Parser) (Name, error)",
		"func(p *jsonbind.Parser) (Level, error)",
		"return Mark(v), nil",
		// The width is still checked, and the bound is still an untyped literal.
		"if v < 0 || v > 65535 {",
		// The encoder converts the other way, at every shape.
		"jsonbind.AppendUint(dst, uint64(v.Marks[i]))",
		"jsonbind.AppendString(dst, string(v.Names[i]))",
		"jsonbind.AppendUint(dst, uint64(v.ByName[k]))",
		// CBOR builds a collection of the declared type, not of the width.
		"make([]Mark, 0, n)",
		"make(map[string]Mark, n)",
	} {
		if !strings.Contains(source, want) {
			t.Errorf("generated source is missing %q", want)
		}
	}
}

// A named element over byte is not a byte sequence: []Mark is not []byte, so it
// stays a list of numbers rather than becoming base64.
func TestANamedByteElementIsNotABlob(t *testing.T) {
	code, err := analyzeOneField(t, "type Mark byte\n", "C []Mark `json:\"c\"`")
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	if strings.Contains(code, "AppendBase64") {
		t.Fatalf("a slice of a named byte was encoded as a blob:\n%s", code)
	}
	if !strings.Contains(code, "jsonbind.AppendUint(dst, uint64(v.C[i]))") {
		t.Fatalf("a slice of a named byte does not encode as numbers:\n%s", code)
	}
}

// The end of the line: the generated code compiles and round-trips a named
// element through JSON and through CBOR, keeping the declared type at both ends.
func TestNamedElementsRoundTripOverHTTP(t *testing.T) {
	opts := generator.DefaultOptions()
	opts.EnableCBORHTTP = true
	dir, code := emitFixedArray(t, namedElementSource, opts)
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

const full = ` + "`" + `{"marks":[1,2],"names":["a","b"],"slots":[7,8,9],"byName":{"x":3}}` + "`" + `

func TestNamedElementsRoundTrip(t *testing.T) {
	w := post(t, full)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var got Result
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	// The declared types survive: these comparisons would not compile against
	// a []uint or a map[string]uint.
	if len(got.Marks) != 2 || got.Marks[0] != Mark(1) || got.Marks[1] != Mark(2) {
		t.Fatalf("marks %v", got.Marks)
	}
	if len(got.Names) != 2 || got.Names[0] != Name("a") {
		t.Fatalf("names %v", got.Names)
	}
	if got.Slots != [3]Level{7, 8, 9} {
		t.Fatalf("slots %v", got.Slots)
	}
	if got.ByName["x"] != Mark(3) {
		t.Fatalf("byName %v", got.ByName)
	}
}

// The width is enforced at the element, as it is for an unnamed one.
func TestAnOutOfRangeNamedElementIs400(t *testing.T) {
	if w := post(t, ` + "`" + `{"slots":[65536]}` + "`" + `); w.Code != http.StatusBadRequest {
		t.Fatalf("status %d, body %s", w.Code, w.Body.String())
	}
}

func TestCBORCarriesTheDeclaredTypes(t *testing.T) {
	in := cbor.AppendMapHeader(nil, 3)
	in = cbor.AppendText(in, "marks")
	in = cbor.AppendArrayHeader(in, 2)
	in = cbor.AppendUint(in, 4)
	in = cbor.AppendUint(in, 5)
	in = cbor.AppendText(in, "slots")
	in = cbor.AppendArrayHeader(in, 2)
	in = cbor.AppendUint(in, 1)
	in = cbor.AppendUint(in, 2)
	in = cbor.AppendText(in, "byName")
	in = cbor.AppendMapHeader(in, 1)
	in = cbor.AppendText(in, "y")
	in = cbor.AppendUint(in, 6)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/boards", bytes.NewReader(in))
	r.Header.Set("Content-Type", "application/cbor")
	Handler(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var got Result
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Marks) != 2 || got.Marks[1] != Mark(5) {
		t.Fatalf("marks %v", got.Marks)
	}
	if got.Slots != [3]Level{1, 2, 0} {
		t.Fatalf("slots %v", got.Slots)
	}
	if got.ByName["y"] != Mark(6) {
		t.Fatalf("byName %v", got.ByName)
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
		t.Fatalf("named elements do not round-trip: %v\n%s\n%s", err, output, code)
	}
	if strings.Contains(string(output), "no test files") {
		t.Fatalf("the probe did not run: %s", output)
	}
}
