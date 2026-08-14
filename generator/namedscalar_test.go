package generator_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

// A named scalar reaches many emission sites: it is bound from the query, the
// path and the body, encoded, decoded, checked, defaulted and enumerated, and
// each of those writes an expression in the underlying kind against a field
// declared in the named one. Go has no implicit conversion between the two, so
// a site that forgets one emits source that does not compile.
//
// Compiling the output is therefore the test. Reading the emitter and hoping
// every site was covered is exactly how the defect this fixes arrived: the
// failure is an undefined identifier inside a file headed DO NOT EDIT, which
// no unit test on a substring would have caught either.
const namedScalarSource = `package main

import (
	"net/http"

	"github.com/shibukawa/tinybind-go"
)

type UserID string
type Count int
type Big int64
type Flag bool
type Ratio float64

type Nested struct {
	Tag UserID ` + "`json:\"tag\"`" + `
}

type Req struct {
	ID     UserID  ` + "`path:\"id\" check:\"minlen=1\"`" + `
	N      Count   ` + "`query:\"n\" default:\"3\"`" + `
	Big    Big     ` + "`query:\"big\"`" + `
	Flag   Flag    ` + "`query:\"flag\"`" + `
	Ratio  Ratio   ` + "`query:\"ratio\"`" + `
	Kind   UserID  ` + "`payload:\"kind\" enum:\"a,b\"`" + `
	Inner  Nested  ` + "`payload:\"inner\"`" + `
	Plain  string  ` + "`query:\"plain\"`" + `
}

type Resp struct {
	ID    UserID ` + "`json:\"id\"`" + `
	N     Count  ` + "`json:\"n\"`" + `
	Inner Nested ` + "`json:\"inner\"`" + `
}

func init() {
	http.HandleFunc("POST /x/{id}", func(w http.ResponseWriter, r *http.Request) {
		in, err := httpbind.Bind[Req](r)
		if err != nil {
			httpbind.WriteError(w, r, err)
			return
		}
		_ = httpbind.Write[Resp](w, r, Resp{ID: in.ID, N: in.N, Inner: in.Inner})
	})
}

func main() {}
`

func TestNamedScalarGeneratesCompilableSource(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(namedScalarSource), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)

	plan, err := generator.AnalyzePackage(dir)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	code, err := generator.Emit(plan)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	// Before the fix, generation named a codec per named scalar and defined
	// none of them.
	for _, absent := range []string{"appendUserIDJSON", "decodeUserIDBytes", "decodeCountBytes"} {
		if strings.Contains(string(code), absent) {
			t.Fatalf("a named scalar must not get a codec of its own; found %q:\n%s", absent, code)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "tinybind_gen.go"), code, 0o644); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "build", "./...")
	command.Dir = dir
	command.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generated source does not compile: %v\n%s\n%s", err, output, code)
	}
}

// analyzeNamedScalar generates over one struct and returns the analysis error.
func analyzeNamedScalar(t *testing.T, body string) error {
	t.Helper()
	dir := t.TempDir()
	writeTempModule(t, dir)
	src := "package main\n\ntype UserID string\ntype Count int\n\n" + body + "\n\nfunc main() {}\n"
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	_, err := generator.AnalyzePackage(dir)
	return err
}

// A named element cannot ride the bulk decoders, which answer a concrete
// []string that Go will not assign to a slice of the named type. It is refused
// rather than emitted wrong, which is what it was before: the same shape used
// to produce a call to a codec nothing defined.
func TestNamedScalarSliceIsRefused(t *testing.T) {
	err := analyzeNamedScalar(t, "type Req struct {\n\tTags []UserID `json:\"tags\"`\n}")
	if err == nil {
		t.Fatal("want an error")
	}
	for _, want := range []string{"Tags", "UserID", "element by element", "[]string"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q missing %q", err, want)
		}
	}
}

func TestNamedScalarMapValueIsRefused(t *testing.T) {
	err := analyzeNamedScalar(t, "type Req struct {\n\tByName map[string]Count `json:\"byName\"`\n}")
	if err == nil {
		t.Fatal("want an error")
	}
	for _, want := range []string{"ByName", "Count", "entry by entry", "map[string]int"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q missing %q", err, want)
		}
	}
}

// A named type this generator can place nothing for is reported rather than
// assumed to be a nested struct, which is what produced a call to a codec
// nothing emitted.
func TestNamedTypeWithAnUnsupportedUnderlyingIsRefused(t *testing.T) {
	err := analyzeNamedScalar(t, "type Weird chan int\n\ntype Req struct {\n\tC Weird `json:\"c\"`\n}")
	if err == nil {
		t.Fatal("want an error")
	}
	for _, want := range []string{"C", "Weird", "cannot map"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q missing %q", err, want)
		}
	}
}

// Compiling is not the whole claim. A named scalar has to appear on the wire as
// what it is underneath — a UserID as a JSON string, not as an object and not
// as a document of its own — which is what encoding/json does with one and what
// an author writing an ID type expects.
func TestNamedScalarRoundTripsAsItsUnderlyingKind(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	src := `package main

import "github.com/shibukawa/tinybind-go/jsonbind"

type UserID string
type Count int

type Req struct {
	ID UserID ` + "`json:\"id\"`" + `
	N  Count  ` + "`json:\"n\"`" + `
}

var _ = jsonbind.GenerateCodec[Req]()

func main() {}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)

	plan, err := generator.AnalyzePackage(dir)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	code, err := generator.Emit(plan)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tinybind_gen.go"), code, 0o644); err != nil {
		t.Fatal(err)
	}
	probe := `package main

import (
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/jsonbind"
)

func TestWire(t *testing.T) {
	var buf strings.Builder
	if err := jsonbind.EncodeJSON(&buf, Req{ID: "u-3", N: 7}); err != nil {
		t.Fatal(err)
	}
	const want = ` + "`" + `{"id":"u-3","n":7}` + "`" + `
	if got := strings.TrimSpace(buf.String()); got != want {
		t.Fatalf("encoded %s, want %s", got, want)
	}
	back, err := jsonbind.DecodeJSONBytes[Req]([]byte(want))
	if err != nil {
		t.Fatal(err)
	}
	if back.ID != "u-3" || back.N != 7 {
		t.Fatalf("round trip gave %+v", back)
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
		t.Fatalf("named scalar does not round trip: %v\n%s\n%s", err, output, code)
	}
}
