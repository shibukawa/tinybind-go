package generator_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

// An error value has no document shape, and silence is not an answer either.
func TestErrorFieldIsRefusedByName(t *testing.T) {
	_, err := analyzeOneField(t, "", "E error `json:\"e\"`")
	if err == nil || !strings.Contains(err.Error(), "field E: an error value has no JSON form") {
		t.Fatalf("unexpected diagnostic: %v", err)
	}
}

// DecodeJSONBytes and DecodeJSONLimit decode the same codec as DecodeJSON, so a
// package reading through either alone must get its decoder.
func TestByteAndLimitDecodeEntriesAreDiscovered(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	src := `package sample

import hb "github.com/shibukawa/tinybind-go/jsonbind"

type Note struct{ Text string }

func use() {
	_, _ = hb.DecodeJSONBytes[Note](nil)
	_, _ = hb.DecodeJSONLimit[Note](nil, 1)
}
`
	writeTestFile(t, filepath.Join(dir, "main.go"), src)
	tidyTempModule(t, dir)
	plan, err := generator.New(generator.DefaultOptions()).Analyze(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Types) != 1 || plan.Types[0].Usage&generator.UsageDecodeJSON == 0 {
		t.Fatalf("the byte/limit decode entries did not mark Note for decoding: %+v", plan.Types)
	}
}

// The compiled half: any values and an empty nested struct round-trip through
// the generated codec and agree with encoding/json on what they mean.
func TestAnyAndEmptyStructRoundTrip(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	src := `package main

import "github.com/shibukawa/tinybind-go/jsonbind"

type Empty struct{ hidden int }

type Doc struct {
	V any            ` + "`json:\"v\"`" + `
	I interface{}    ` + "`json:\"i\"`" + `
	L []any          ` + "`json:\"l\"`" + `
	M map[string]any ` + "`json:\"m\"`" + `
	E Empty          ` + "`json:\"e\"`" + `
	O any            ` + "`json:\"o,omitempty\"`" + `
}

var _ = jsonbind.GenerateCodec[Doc]()

func main() {}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	plan, err := generator.AnalyzePackageWithOptions(dir, generator.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	code, err := generator.Emit(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tinybind_gen.go"), code, 0o644); err != nil {
		t.Fatal(err)
	}
	probe := `package main

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/shibukawa/tinybind-go/jsonbind"
)

const doc = ` + "`" + `{"v":{"a":[1,"b",null,true]},"i":"s","l":[1,{"k":2.5},[]],"m":{"z":null,"a":1},"e":{"ignored":1}}` + "`" + `

func TestRoundTrip(t *testing.T) {
	got, err := jsonbind.DecodeJSONBytes[Doc]([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	var want Doc
	if err := json.Unmarshal([]byte(doc), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decode differs\n got %#v\nwant %#v", got, want)
	}
	var buf bytes.Buffer
	if err := jsonbind.EncodeJSON(&buf, got); err != nil {
		t.Fatal(err)
	}
	var mine, theirs any
	if err := json.Unmarshal(buf.Bytes(), &mine); err != nil {
		t.Fatalf("%v: %s", err, buf.Bytes())
	}
	std, _ := json.Marshal(want)
	if err := json.Unmarshal(std, &theirs); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(mine, theirs) {
		t.Fatalf("encode differs\n mine %s\ntheirs %s", buf.Bytes(), std)
	}
	if bytes.Contains(buf.Bytes(), []byte(` + "`" + `"o"` + "`" + `)) {
		t.Fatalf("a nil any was not omitted: %s", buf.Bytes())
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
		t.Fatalf("any fields do not round-trip: %v\n%s\n%s", err, output, code)
	}
}
