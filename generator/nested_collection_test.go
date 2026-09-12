package generator_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

// A shape the generator cannot map is refused by name, not dropped.
func TestUnmappableShapesAreRefusedByName(t *testing.T) {
	for _, tc := range []struct{ field, want string }{
		{"C chan int `json:\"c\"`", "cannot map chan int"},
		{"M map[int]string `json:\"m\"`", "string key only"},
		{"F func() `json:\"f\"`", "cannot map func()"},
	} {
		_, err := analyzeOneField(t, "", tc.field)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: unexpected diagnostic: %v", tc.field, err)
		}
	}
}

// The compiled half: a tree through pointers, nested collections and a
// spaced name round-trip and agree with encoding/json.
func TestPointersAndNestedCollectionsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	src := `package main

import "github.com/shibukawa/tinybind-go/jsonbind"

type Node struct {
	Name string  ` + "`json:\"name\"`" + `
	Kids []*Node ` + "`json:\"kids\"`" + `
	Next *Node   ` + "`json:\"next\"`" + `
}

type Doc struct {
	Tree   *Node               ` + "`json:\"tree\"`" + `
	Grid   [][]int             ` + "`json:\"grid\"`" + `
	Coords [][2]float64        ` + "`json:\"coords\"`" + `
	Tags   map[string][]string ` + "`json:\"tags\"`" + `
	Rows   []map[string]string ` + "`json:\"rows\"`" + `
	Box    string              ` + "`json:\"Box Drawing\"`" + `
	Count  *int                ` + "`json:\"count\"`" + `
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

const doc = ` + "`" + `{"tree":{"name":"root","kids":[{"name":"a","kids":[],"next":null},null],"next":{"name":"n","kids":null,"next":null}},"grid":[[1,2],[],[3]],"coords":[[1.5,2],[3,4.25]],"tags":{"x":["a","b"],"y":[]},"rows":[{"k":"v"},{}],"Box Drawing":"╔","count":7}` + "`" + `

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
	// encoding/json writes a nil slice as null and this codec as [], per
	// the documented divergence; normalise that one difference.
	if !reflect.DeepEqual(nullsToEmpty(mine), nullsToEmpty(theirs)) {
		t.Fatalf("encode differs\n mine %s\ntheirs %s", buf.Bytes(), std)
	}
}

func nullsToEmpty(v any) any {
	switch x := v.(type) {
	case map[string]any:
		for k, e := range x {
			if e == nil && (k == "kids" || k == "grid" || k == "tags" || k == "rows") {
				x[k] = []any{}
			} else {
				x[k] = nullsToEmpty(e)
			}
		}
		return x
	case []any:
		for i := range x {
			x[i] = nullsToEmpty(x[i])
		}
	}
	return v
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
		t.Fatalf("pointers and nested collections do not round-trip: %v\n%s\n%s", err, output, code)
	}
}
