package generator_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

// writeForeignFixture lays out two packages: domain declares a type carrying
// its own JSON codec, and the analyzed package holds it as a field. Before the
// interfaces, such a field was dropped from the plan without a word, because
// analysis is per package and a qualified type name resolves to nothing it can
// walk.
func writeForeignFixture(t *testing.T, fieldTag string) string {
	t.Helper()
	dir := t.TempDir()
	writeTempModule(t, dir)
	if err := os.Mkdir(filepath.Join(dir, "domain"), 0o755); err != nil {
		t.Fatal(err)
	}
	domain := `package domain

import "github.com/shibukawa/tinybind-go/jsonbind"

type Point struct {
	X int
	Y int
}

func (p Point) AppendJSONTo(dst []byte) []byte {
	dst = append(dst, ` + "`" + `{"x":` + "`" + `...)
	dst = jsonbind.AppendInt(dst, int64(p.X))
	dst = append(dst, ` + "`" + `,"y":` + "`" + `...)
	dst = jsonbind.AppendInt(dst, int64(p.Y))
	return append(dst, '}')
}

func (p *Point) DecodeJSONFrom(data []byte) error {
	m, err := jsonbind.DecodeJSONMapStringInt(data)
	if err != nil {
		return err
	}
	p.X, p.Y = m["x"], m["y"]
	return nil
}
`
	if err := os.WriteFile(filepath.Join(dir, "domain", "domain.go"), []byte(domain), 0o644); err != nil {
		t.Fatal(err)
	}
	main := `package main

import (
	"github.com/shibukawa/tinybind-go/jsonbind"
	"tempmod/domain"
)

type Marker struct {
	Label  string       ` + "`json:\"label\"`" + `
	Origin domain.Point ` + fieldTag + `
}

var _ = jsonbind.GenerateCodec[Marker]()

func main() {}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(main), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	return dir
}

func TestForeignFieldIsEncodedThroughItsOwnCodec(t *testing.T) {
	dir := writeForeignFixture(t, "`json:\"origin\"`")
	plan, err := generator.AnalyzePackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	code, err := generator.Emit(plan)
	if err != nil {
		t.Fatal(err)
	}
	source := string(code)
	for _, want := range []string{
		"dst = v.Origin.AppendJSONTo(dst)",
		"out.Origin.DecodeJSONFrom(",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("generated code missing %q:\n%s", want, source)
		}
	}
	if strings.Contains(source, "appendPointJSON") {
		t.Fatalf("a foreign type must not get a generated codec:\n%s", source)
	}
}

// The generated codec has to compile against the foreign package and round-trip
// the whole document, which is what proves the field is really carried rather
// than merely mentioned.
func TestForeignFieldRoundTrips(t *testing.T) {
	dir := writeForeignFixture(t, "`json:\"origin\"`")
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
	probe := `package main

import (
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/jsonbind"
	"tempmod/domain"
)

func TestRoundTrip(t *testing.T) {
	var buf strings.Builder
	in := Marker{Label: "home", Origin: domain.Point{X: 1, Y: 2}}
	if err := jsonbind.EncodeJSON(&buf, in); err != nil {
		t.Fatal(err)
	}
	const want = ` + "`" + `{"label":"home","origin":{"x":1,"y":2}}` + "`" + `
	if got := buf.String(); got != want {
		t.Fatalf("encoded %s, want %s", got, want)
	}
	out, err := jsonbind.DecodeJSONBytes[Marker]([]byte(want))
	if err != nil {
		t.Fatal(err)
	}
	if out != in {
		t.Fatalf("round trip gave %+v, want %+v", out, in)
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
		t.Fatalf("foreign field does not round trip: %v\n%s\n%s", err, output, code)
	}
}

// Both options ask this module whether the value is empty, which it cannot know
// for a type whose shape it never read.
func TestForeignFieldRefusesOmitEmpty(t *testing.T) {
	dir := writeForeignFixture(t, "`json:\"origin,omitempty\"`")
	_, err := generator.AnalyzePackage(dir)
	if err == nil {
		t.Fatal("want an error")
	}
	if !strings.Contains(err.Error(), "omitempty") || !strings.Contains(err.Error(), "carries its own JSON codec") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// A foreign type carrying neither method stays dropped, exactly as before.
func TestForeignFieldWithoutTheMethodsIsStillDropped(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	if err := os.Mkdir(filepath.Join(dir, "domain"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "domain", "domain.go"),
		[]byte("package domain\n\ntype Plain struct{ X int }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	main := `package main

import (
	"github.com/shibukawa/tinybind-go/jsonbind"
	"tempmod/domain"
)

type Marker struct {
	Label string       ` + "`json:\"label\"`" + `
	Plain domain.Plain ` + "`json:\"plain\"`" + `
}

var _ = jsonbind.GenerateCodec[Marker]()

func main() {}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(main), 0o644); err != nil {
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
	if strings.Contains(string(code), "Plain") {
		t.Fatalf("a type carrying no codec must stay dropped:\n%s", code)
	}
}

// writeOneDirectionalFixture is writeForeignFixture with the foreign type
// carrying only the named half, and the parent asking for only the named
// annotation.
func writeOneDirectionalFixture(t *testing.T, half, annotation string) string {
	t.Helper()
	dir := t.TempDir()
	writeTempModule(t, dir)
	if err := os.Mkdir(filepath.Join(dir, "domain"), 0o755); err != nil {
		t.Fatal(err)
	}
	appendHalf := `
func (p Point) AppendJSONTo(dst []byte) []byte {
	dst = append(dst, ` + "`" + `{"x":` + "`" + `...)
	dst = jsonbind.AppendInt(dst, int64(p.X))
	return append(dst, '}')
}
`
	decodeHalf := `
func (p *Point) DecodeJSONFrom(data []byte) error {
	m, err := jsonbind.DecodeJSONMapStringInt(data)
	if err != nil {
		return err
	}
	p.X = m["x"]
	return nil
}
`
	methods := appendHalf
	if half == "decode" {
		methods = decodeHalf
	}
	domain := "package domain\n\nimport \"github.com/shibukawa/tinybind-go/jsonbind\"\n\ntype Point struct{ X int }\n" + methods
	if err := os.WriteFile(filepath.Join(dir, "domain", "domain.go"), []byte(domain), 0o644); err != nil {
		t.Fatal(err)
	}
	main := `package main

import (
	"github.com/shibukawa/tinybind-go/jsonbind"
	"tempmod/domain"
)

type Marker struct {
	Label  string       ` + "`json:\"label\"`" + `
	Origin domain.Point ` + "`json:\"origin\"`" + `
}

var _ = jsonbind.` + annotation + `[Marker]()

func main() {}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(main), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	return dir
}

func emitOneDirectional(t *testing.T, half, annotation string) (string, error) {
	t.Helper()
	dir := writeOneDirectionalFixture(t, half, annotation)
	opts := generator.DefaultOptions()
	plan, err := generator.New(opts).Analyze(dir)
	if err != nil {
		t.Fatal(err)
	}
	code, err := generator.Emit(plan)
	return string(code), err
}

// A foreign type carrying one half is usable for as long as its parent only
// needs that half. Which half a field needs follows from the parent's usage,
// which nothing knows while the field is being admitted, so admission takes
// what the type has and the requirement is checked here instead.
func TestForeignFieldWithOnlyTheEncodeHalfIsEncoded(t *testing.T) {
	source, err := emitOneDirectional(t, "append", "GenerateEncoder")
	if err != nil {
		t.Fatalf("an encode-only parent must accept an encode-only field: %v", err)
	}
	if !strings.Contains(source, "dst = v.Origin.AppendJSONTo(dst)") {
		t.Fatalf("generated code missing the append call:\n%s", source)
	}
	if strings.Contains(source, "DecodeJSONFrom") {
		t.Fatalf("nothing should decode here:\n%s", source)
	}
}

func TestForeignFieldWithOnlyTheDecodeHalfIsDecoded(t *testing.T) {
	source, err := emitOneDirectional(t, "decode", "GenerateDecoder")
	if err != nil {
		t.Fatalf("a decode-only parent must accept a decode-only field: %v", err)
	}
	if !strings.Contains(source, "Origin.DecodeJSONFrom(") {
		t.Fatalf("generated code missing the decode call:\n%s", source)
	}
	if strings.Contains(source, "AppendJSONTo") {
		t.Fatalf("nothing should encode here:\n%s", source)
	}
}

// The refusal is a diagnostic naming the field and the missing method. Without
// it the emission would name a call the type does not carry, and the error
// would land as a compile failure inside a DO NOT EDIT file.
func TestForeignFieldMissingTheHalfItsParentNeedsIsReported(t *testing.T) {
	_, err := emitOneDirectional(t, "append", "GenerateCodec")
	if err == nil {
		t.Fatal("want an error")
	}
	for _, want := range []string{"Marker.Origin", "domain.Point", "DecodeJSONFrom", "decoded"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q missing %q", err, want)
		}
	}
}

func TestForeignFieldMissingTheEncodeHalfItsParentNeedsIsReported(t *testing.T) {
	_, err := emitOneDirectional(t, "decode", "GenerateCodec")
	if err == nil {
		t.Fatal("want an error")
	}
	for _, want := range []string{"Marker.Origin", "domain.Point", "AppendJSONTo", "encoded"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q missing %q", err, want)
		}
	}
}
