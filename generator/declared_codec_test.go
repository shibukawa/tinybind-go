package generator_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

// emitDeclaredCodecSource generates over one file whose only mention of the
// types is the annotations it carries.
func emitDeclaredCodecSource(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	writeTempModule(t, dir)
	src := `package sample

import "github.com/shibukawa/tinybind-go/jsonbind"

type Point struct {
	X int ` + "`json:\"x\"`" + `
	Y int ` + "`json:\"y\"`" + `
}

type Label struct {
	Text string ` + "`json:\"text\"`" + `
}

` + body + `
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)

	plan, err := generator.AnalyzePackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	code, err := generator.Emit(plan)
	if err != nil {
		t.Fatal(err)
	}
	return string(code)
}

func mustContainAll(t *testing.T, source string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(source, w) {
			t.Fatalf("generated code missing %q:\n%s", w, source)
		}
	}
}

func mustContainNone(t *testing.T, source string, unwanted ...string) {
	t.Helper()
	for _, w := range unwanted {
		if strings.Contains(source, w) {
			t.Fatalf("generated code should not contain %q:\n%s", w, source)
		}
	}
}

// A type no call site names gets a codec because an annotation asked for it.
// Before this, generation was driven entirely by discovered calls and this file
// produced nothing at all.
func TestGenerateCodecEmitsBothDirectionsAndBothMethods(t *testing.T) {
	source := emitDeclaredCodecSource(t, `var _ = jsonbind.GenerateCodec[Point]()`)
	mustContainAll(t, source,
		"func appendPointJSON(dst []byte, v Point) []byte",
		"func decodePointBytes(data []byte) (Point, error)",
		"func (v Point) AppendJSONTo(dst []byte) []byte",
		"func (v *Point) DecodeJSONFrom(data []byte) error",
		"return appendPointJSON(dst, v)",
	)
}

// The annotation names the direction, so the published method set follows it.
// This has to come from the annotation rather than from the type's overall
// usage: GenerateAll gives every type every codec, and reading the direction
// off that would publish both methods for a type that asked for one.
func TestGenerateEncoderPublishesTheEncodeMethodOnly(t *testing.T) {
	source := emitDeclaredCodecSource(t, `var _ = jsonbind.GenerateEncoder[Point]()`)
	mustContainAll(t, source, "func (v Point) AppendJSONTo(dst []byte) []byte")
	mustContainNone(t, source, "DecodeJSONFrom")
}

func TestGenerateDecoderPublishesTheDecodeMethodOnly(t *testing.T) {
	source := emitDeclaredCodecSource(t, `var _ = jsonbind.GenerateDecoder[Point]()`)
	mustContainAll(t, source, "func (v *Point) DecodeJSONFrom(data []byte) error")
	mustContainNone(t, source, "AppendJSONTo")
}

// The annotation is what publishes the methods. A codec reached through an
// ordinary call site emits the functions alone, so an existing project's output
// does not move.
func TestADiscoveredCallEmitsNoMethods(t *testing.T) {
	source := emitDeclaredCodecSource(t, `
func read(data []byte) Point {
	p, _ := jsonbind.DecodeJSONBytes[Point](data)
	return p
}`)
	mustContainAll(t, source, "func decodePointBytes(data []byte) (Point, error)")
	mustContainNone(t, source, "DecodeJSONFrom", "AppendJSONTo")
}

// An annotation names one type, so a type beside it publishes nothing even
// though generation reached it.
func TestAnUnannotatedTypeBesideAnAnnotatedOnePublishesNothing(t *testing.T) {
	source := emitDeclaredCodecSource(t, `var _ = jsonbind.GenerateCodec[Point]()`)
	mustContainAll(t, source, "func (v Point) AppendJSONTo(dst []byte) []byte")
	mustContainNone(t, source,
		"func (v Label) AppendJSONTo",
		"func (v *Label) DecodeJSONFrom",
	)
}

// Disabling one codec direction has to leave the other half of a both-directions
// annotation standing. A pattern is the unit a disabled feature removes, which
// is why GenerateCodec is registered as two patterns rather than as one
// operation carrying both directions: with one, disabling DecodeJSON either
// took the encoder with it or left the decoder enabled through the annotation.
func TestDisablingOneDirectionLeavesTheOtherHalfOfAnAnnotation(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	src := `package sample

import "github.com/shibukawa/tinybind-go/jsonbind"

type Point struct {
	X int ` + "`json:\"x\"`" + `
}

var _ = jsonbind.GenerateCodec[Point]()
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)

	opts := generator.DefaultOptions()
	opts.DisableFeatures = append(opts.DisableFeatures, generator.FeatureDecodeJSON)
	plan, err := generator.New(opts).Analyze(dir)
	if err != nil {
		t.Fatal(err)
	}
	code, err := generator.Emit(plan)
	if err != nil {
		t.Fatal(err)
	}
	source := string(code)
	mustContainAll(t, source, "func (v Point) AppendJSONTo(dst []byte) []byte")
	mustContainNone(t, source, "DecodeJSONFrom", "func decodePointBytes")
}

// The point of publishing the methods is that the interfaces are satisfied
// with no hand-written code, so the assertions below are the acceptance
// criterion rather than a smoke test: they fail to compile if the emitted
// method set drifts from what jsonbind declares.
func TestGeneratedMethodsSatisfyTheInterfaces(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	src := `package main

import "github.com/shibukawa/tinybind-go/jsonbind"

type Point struct {
	X int ` + "`json:\"x\"`" + `
	Y int ` + "`json:\"y\"`" + `
}

var _ = jsonbind.GenerateCodec[Point]()

var (
	_ jsonbind.Appender = Point{}
	_ jsonbind.Decoder  = (*Point)(nil)
)

func main() {}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)

	plan, err := generator.AnalyzePackage(dir)
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
	command := exec.Command("go", "build", "./...")
	command.Dir = dir
	command.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generated codec does not satisfy the interfaces: %v\n%s\n%s", err, output, code)
	}
}
