package generator_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

// aliasFieldSource reaches every shape resolveNamedKind has to tell apart: an
// alias of a predeclared scalar, an alias of a same-package defined scalar, an
// alias of a same-package struct, and a slice and a fixed-length array of the
// first. One fixture covers the binder, the JSON codec and the CBOR codec at
// once, over both a scalar and a composite alias.
const aliasFieldSource = `package main

import (
	"net/http"

	"github.com/shibukawa/tinybind-go"
)

type Mark = uint
type Level uint16
type M = Level

type Mood struct {
	X int ` + "`json:\"x\"`" + `
}
type Feeling = Mood

type Doc struct {
	Scalar  Mark    ` + "`payload:\"scalar\"`" + `
	Defined M       ` + "`payload:\"defined\"`" + `
	Struct  Feeling ` + "`payload:\"struct\"`" + `
	Slice   []Mark  ` + "`payload:\"slice\"`" + `
	Array   [3]Mark ` + "`payload:\"array\"`" + `
}

type Result struct {
	Scalar  Mark    ` + "`json:\"scalar\"`" + `
	Defined M       ` + "`json:\"defined\"`" + `
	Struct  Feeling ` + "`json:\"struct\"`" + `
	Slice   []Mark  ` + "`json:\"slice\"`" + `
	Array   [3]Mark ` + "`json:\"array\"`" + `
}

func Handler(w http.ResponseWriter, r *http.Request) {
	in, err := httpbind.Bind[Doc](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	_ = httpbind.Write[Result](w, r, Result(in))
}

func main() {
	http.HandleFunc("POST /doc", Handler)
}
`

func emitAliasFields(t *testing.T, src string, opts generator.Options) (string, string) {
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

// The regression this exists for: type Mark = uint is an alias, which since
// Go 1.24's gotypesalias=1 default arrives as *types.Alias and fails a
// *types.Named assertion, so it fell through every scalar name and was
// refused -- with a message that said Mark was Mark underneath.
func TestAliasOfPredeclaredScalarIsAdmitted(t *testing.T) {
	_, code := emitAliasFields(t, aliasFieldSource, generator.DefaultOptions())
	if !strings.Contains(code, "v.Scalar") {
		t.Fatalf("the alias field was not planned:\n%s", code)
	}
}

// The acceptance requirement:alias-transparent-type-analysis states for every
// site it covers: an alias and the original spelling of one type generate
// byte-identical code. For a predeclared scalar that means no conversion at
// all -- the same code a bare uint field would generate.
func TestAliasOfPredeclaredScalarIsByteIdenticalToTheOriginal(t *testing.T) {
	aliasSrc := `package main

import "github.com/shibukawa/tinybind-go/jsonbind"

type Mark = uint

type Doc struct {
	C Mark ` + "`json:\"c\"`" + `
}

var _ = jsonbind.GenerateCodec[Doc]()

func main() {}
`
	directSrc := `package main

import "github.com/shibukawa/tinybind-go/jsonbind"

type Doc struct {
	C uint ` + "`json:\"c\"`" + `
}

var _ = jsonbind.GenerateCodec[Doc]()

func main() {}
`
	_, aliasCode := emitAliasFields(t, aliasSrc, generator.DefaultOptions())
	_, directCode := emitAliasFields(t, directSrc, generator.DefaultOptions())
	if aliasCode != directCode {
		t.Fatalf("alias and direct spelling are not byte-identical\nALIAS:\n%s\nDIRECT:\n%s", aliasCode, directCode)
	}
}

// type M = Level (with type Level uint16) resolves through the alias to
// Level's own *types.Named. Generated code has to spell the conversion with
// Level's own declared name, not M: M is not a real type declaration, so a
// generated file could not import or otherwise name "M" as a type on its own
// terms the way it can Level -- and after Unalias runs, that is what the
// analysis knows the field is.
func TestAliasOfDefinedScalarUsesTheResolvedName(t *testing.T) {
	_, code := emitAliasFields(t, aliasFieldSource, generator.DefaultOptions())
	if !strings.Contains(code, "Level(") {
		t.Fatalf("the alias element does not carry the resolved declared name:\n%s", code)
	}
}

// fieldTypeKind used to plan a KindStruct field under the identifier it was
// written with, which is safe for a direct field (the identifier names the
// struct) but not for an alias: type Feeling = Mood, used as a field, must
// call decodeMoodJSON, because that is the function this package actually
// generates. Calling decodeFeelingJSON would be a reference to a function
// nothing defines, inside a file headed DO NOT EDIT.
func TestAliasOfLocalStructCallsTheRealDecoder(t *testing.T) {
	_, code := emitAliasFields(t, aliasFieldSource, generator.DefaultOptions())
	if !strings.Contains(code, "decodeMoodJSON") {
		t.Fatalf("the struct alias does not call decodeMoodJSON:\n%s", code)
	}
	if strings.Contains(code, "decodeFeelingJSON") || strings.Contains(code, "appendFeelingJSON") {
		t.Fatalf("the struct alias called a function nothing defines:\n%s", code)
	}
}

// An alias reaching into another package is refused rather than planned as a
// same-package struct this run does not declare. Generation is per package,
// per rule:same-package-convention; a foreign struct reached any other way is
// already refused on the same grounds.
func TestAliasOfForeignTypeIsRefused(t *testing.T) {
	src := `package main

import (
	"net/http"

	"github.com/shibukawa/tinybind-go/jsonbind"
)

type M = http.Header

type Doc struct {
	C M ` + "`json:\"c\"`" + `
}

var _ = jsonbind.GenerateCodec[Doc]()

func main() {}
`
	dir := t.TempDir()
	writeTempModule(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	_, err := generator.AnalyzePackageWithOptions(dir, generator.DefaultOptions())
	if err == nil {
		t.Fatal("an alias of a foreign type was accepted")
	}
	for _, want := range []string{"C", "M", "net/http", "package boundary"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("refusal is missing %q: %v", want, err)
		}
	}
}

// The end of the line: every alias shape above compiles, binds over HTTP, and
// round-trips through JSON and through CBOR, keeping the resolved types intact
// at both ends.
func TestAliasFieldsRoundTripOverHTTP(t *testing.T) {
	opts := generator.DefaultOptions()
	opts.EnableCBORHTTP = true
	dir, code := emitAliasFields(t, aliasFieldSource, opts)
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
	r := httptest.NewRequest(http.MethodPost, "/doc", strings.NewReader(payload))
	r.Header.Set("Content-Type", "application/json")
	Handler(w, r)
	return w
}

const full = ` + "`" + `{"scalar":7,"defined":42,"struct":{"x":9},"slice":[1,2],"array":[3,4,5]}` + "`" + `

func TestAliasFieldsRoundTrip(t *testing.T) {
	w := post(t, full)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var got Result
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	// Every comparison below only compiles because the alias field really did
	// resolve to the type it names: Mark is uint, M is Level, Feeling is Mood.
	if got.Scalar != Mark(7) {
		t.Fatalf("scalar %v", got.Scalar)
	}
	if got.Defined != M(42) {
		t.Fatalf("defined %v", got.Defined)
	}
	if got.Struct != (Feeling{X: 9}) {
		t.Fatalf("struct %v", got.Struct)
	}
	if len(got.Slice) != 2 || got.Slice[0] != Mark(1) || got.Slice[1] != Mark(2) {
		t.Fatalf("slice %v", got.Slice)
	}
	if got.Array != [3]Mark{3, 4, 5} {
		t.Fatalf("array %v", got.Array)
	}
}

func TestAliasFieldsRoundTripOverCBOR(t *testing.T) {
	in := cbor.AppendMapHeader(nil, 5)
	in = cbor.AppendText(in, "scalar")
	in = cbor.AppendUint(in, 7)
	in = cbor.AppendText(in, "defined")
	in = cbor.AppendUint(in, 42)
	in = cbor.AppendText(in, "struct")
	in = cbor.AppendMapHeader(in, 1)
	in = cbor.AppendText(in, "x")
	in = cbor.AppendInt(in, 9)
	in = cbor.AppendText(in, "slice")
	in = cbor.AppendArrayHeader(in, 2)
	in = cbor.AppendUint(in, 1)
	in = cbor.AppendUint(in, 2)
	in = cbor.AppendText(in, "array")
	in = cbor.AppendArrayHeader(in, 3)
	in = cbor.AppendUint(in, 3)
	in = cbor.AppendUint(in, 4)
	in = cbor.AppendUint(in, 5)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/doc", bytes.NewReader(in))
	r.Header.Set("Content-Type", "application/cbor")
	Handler(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var got Result
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Scalar != Mark(7) || got.Defined != M(42) || got.Struct != (Feeling{X: 9}) {
		t.Fatalf("got %+v", got)
	}
	if got.Array != [3]Mark{3, 4, 5} {
		t.Fatalf("array %v", got.Array)
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
		t.Fatalf("alias fields do not round-trip: %v\n%s\n%s", err, output, code)
	}
	if strings.Contains(string(output), "no test files") {
		t.Fatalf("the probe did not run: %s", output)
	}
}
