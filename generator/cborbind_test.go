package generator_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

// cborBindSource calls three of the four entry points, so one fixture covers
// both shapes, a narrowed direction, and a nested struct reached from a root.
const cborBindSource = `package main

import (
	"github.com/shibukawa/tinybind-go/cborbind"
)

type Vec struct {
	X int16 ` + "`json:\"x\"`" + `
	Y int16 ` + "`json:\"y\"`" + `
}

type PlayerInput struct {
	Tick uint32 ` + "`json:\"tick\"`" + `
	Aim  Vec    ` + "`json:\"aim\"`" + `
	Btn  uint8  ` + "`json:\"btn\"`" + `
}

type Snapshot struct {
	Round uint16   ` + "`json:\"round\"`" + `
	Names []string ` + "`json:\"names\"`" + `
}

// Sent only, so the array codec is encode-only and nothing below it decodes.
type Telemetry struct {
	Frames uint32 ` + "`json:\"frames\"`" + `
}

var buf []byte

func main() {
	buf = cborbind.AppendCBORInArrayTo(buf[:0], PlayerInput{Tick: 1})
	if in, err := cborbind.DecodeCBORInArrayFrom[PlayerInput](buf); err == nil {
		_ = in
	}
	buf = cborbind.AppendCBORInMapTo(buf[:0], Snapshot{Round: 2})
	if s, err := cborbind.DecodeCBORInMapFrom[Snapshot](buf); err == nil {
		_ = s
	}
	buf = cborbind.AppendCBORInArrayTo(buf[:0], Telemetry{Frames: 3})
}
`

func emitCBORBind(t *testing.T, src string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	writeTempModule(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	plan, err := generator.AnalyzePackageWithOptions(dir, generator.DefaultOptions())
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	code, err := generator.Emit(plan)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	return dir, string(code)
}

// A call is the ask: no declaration was written and the codecs exist.
func TestCBORBindCallsGenerateTheCodecs(t *testing.T) {
	_, source := emitCBORBind(t, cborBindSource)
	for _, want := range []string{
		"func appendPlayerInputCBORArray(dst []byte, v PlayerInput) []byte",
		"func decodePlayerInputCBORArray(cr *cbor.Reader) (PlayerInput, error)",
		"func appendSnapshotCBORMap(dst []byte, v Snapshot) []byte",
		"func decodeSnapshotCBORMap(cr *cbor.Reader) (Snapshot, error)",
		"func (v PlayerInput) AppendCBORInArrayTo(dst []byte) []byte",
		"func (v *PlayerInput) DecodeCBORInArrayFrom(data []byte) error",
		"func (v Snapshot) AppendCBORInMapTo(dst []byte) []byte",
	} {
		if !strings.Contains(source, want) {
			t.Errorf("generated source is missing %q", want)
		}
	}
}

// The shapes differ in what they put on the wire, which is the whole reason
// there are two of them.
func TestCBORBindShapesDiffer(t *testing.T) {
	_, source := emitCBORBind(t, cborBindSource)
	array := codecBody(t, source, "func appendPlayerInputCBORArray")
	if !strings.Contains(array, "cbor.AppendArrayHeader(dst, 3)") {
		t.Errorf("the array codec does not write an array header:\n%s", array)
	}
	if strings.Contains(array, "cbor.AppendText(dst, \"tick\")") {
		t.Error("the array codec puts member names on the wire")
	}
	mapped := codecBody(t, source, "func appendSnapshotCBORMap")
	if !strings.Contains(mapped, "cbor.AppendMapHeader(dst, 2)") {
		t.Errorf("the map codec does not write a map header:\n%s", mapped)
	}
	if !strings.Contains(mapped, "cbor.AppendText(dst, \"names\")") {
		t.Error("the map codec does not key its members")
	}
	// The map decoder skips what it does not know; the array decoder cannot.
	if !strings.Contains(codecBody(t, source, "func decodeSnapshotCBORMap"), "cr.Skip()") {
		t.Error("the map decoder does not skip an unknown key")
	}
	if !strings.Contains(codecBody(t, source, "func decodePlayerInputCBORArray"), "n != 3") {
		t.Error("the array decoder does not pin the member count")
	}
}

// A nested struct is keyed by the shape it was reached from, and an
// encode-only root carries no decoder at any depth.
func TestCBORBindNarrowsShapeAndDirection(t *testing.T) {
	_, source := emitCBORBind(t, cborBindSource)
	if !strings.Contains(source, "func appendVecCBORArray") {
		t.Error("a nested struct reached from an array root has no array codec")
	}
	if strings.Contains(source, "func appendVecCBORMap") {
		t.Error("a nested struct gained a shape no root asked for")
	}
	if !strings.Contains(source, "func appendTelemetryCBORArray") {
		t.Error("the encode-only type has no encoder")
	}
	if strings.Contains(source, "func decodeTelemetryCBORArray") {
		t.Error("an encode-only call emitted a decoder")
	}
	if strings.Contains(source, "func (v *Telemetry) DecodeCBORInArrayFrom") {
		t.Error("an encode-only call published a decode method")
	}
}

// One shape delegates to the driver's own pair; a type is reachable by any
// consumer holding a cbor.Appender.
func TestCBORBindDelegatesToTheDriverInterface(t *testing.T) {
	_, source := emitCBORBind(t, cborBindSource)
	if !strings.Contains(source, "func (v PlayerInput) AppendCBORTo(dst []byte) []byte") {
		t.Error("a single-shape type does not satisfy cbor.Appender")
	}
	if !strings.Contains(source, "func (v *PlayerInput) DecodeCBORFrom(data []byte) error") {
		t.Error("a single-shape type does not satisfy cbor.Decodable")
	}
}

// A type named from both shapes carries both methods and no delegating pair,
// since there is no unambiguous target for one.
func TestCBORBindBothShapesOnOneType(t *testing.T) {
	src := strings.Replace(cborBindSource,
		"buf = cborbind.AppendCBORInArrayTo(buf[:0], Telemetry{Frames: 3})",
		"buf = cborbind.AppendCBORInArrayTo(buf[:0], Telemetry{Frames: 3})\n\tbuf = cborbind.AppendCBORInMapTo(buf[:0], Telemetry{Frames: 3})", 1)
	_, source := emitCBORBind(t, src)
	if !strings.Contains(source, "func (v Telemetry) AppendCBORInArrayTo") ||
		!strings.Contains(source, "func (v Telemetry) AppendCBORInMapTo") {
		t.Error("a two-shape type is missing one of its methods")
	}
	if strings.Contains(source, "func (v Telemetry) AppendCBORTo") {
		t.Error("a two-shape type resolved the ambiguity instead of leaving it visible")
	}
}

// A project calling no entry point emits no CBOR at all, which is the size
// guarantee usage-directed generation gives every other mode.
func TestCBORBindOffEmitsNothing(t *testing.T) {
	src := `package main

type Unused struct {
	A int ` + "`json:\"a\"`" + `
}

func main() {}
`
	_, source := emitCBORBind(t, src)
	mustContainNone(t, source, "CBORArray", "CBORMap", "cborbind.", "AppendCBORInArrayTo")
}

// The end of the line: the generated methods make the entry points compile,
// and both shapes round-trip.
func TestCBORBindRoundTripsCompiled(t *testing.T) {
	dir, code := emitCBORBind(t, cborBindSource)
	if err := os.WriteFile(filepath.Join(dir, "tinybind_gen.go"), []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	probe := `package main

import (
	"testing"

	"github.com/shibukawa/tinybind-go/cborbind"
	"github.com/shibukawa/tinygodriver/encoding/cbor"
)

func TestArrayRoundTrip(t *testing.T) {
	want := PlayerInput{Tick: 4294967295, Aim: Vec{X: -32768, Y: 32767}, Btn: 255}
	enc := cborbind.AppendCBORInArrayTo(nil, want)
	got, err := cborbind.DecodeCBORInArrayFrom[PlayerInput](enc)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestMapRoundTrip(t *testing.T) {
	want := Snapshot{Round: 7, Names: []string{"ada", "grace"}}
	enc := cborbind.AppendCBORInMapTo(nil, want)
	got, err := cborbind.DecodeCBORInMapFrom[Snapshot](enc)
	if err != nil {
		t.Fatal(err)
	}
	if got.Round != want.Round || len(got.Names) != 2 || got.Names[1] != "grace" {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

// The map shape is the evolvable one: a member the build does not know is
// skipped rather than refused.
func TestMapSkipsAnUnknownMember(t *testing.T) {
	enc := cbor.AppendMapHeader(nil, 3)
	enc = cbor.AppendText(enc, "names")
	enc = cbor.AppendArrayHeader(enc, 1)
	enc = cbor.AppendText(enc, "ada")
	enc = cbor.AppendText(enc, "round")
	enc = cbor.AppendUint(enc, 7)
	enc = cbor.AppendText(enc, "combo")
	enc = cbor.AppendUint(enc, 9)
	got, err := cborbind.DecodeCBORInMapFrom[Snapshot](enc)
	if err != nil {
		t.Fatalf("an unknown member was not skipped: %v", err)
	}
	if got.Round != 7 || len(got.Names) != 1 {
		t.Fatalf("got %+v", got)
	}
}

// The array shape is the frozen one: a different length is refused, because it
// cannot tell an added field from a reordered one.
func TestArrayRefusesADifferentLength(t *testing.T) {
	enc := cbor.AppendArrayHeader(nil, 2)
	enc = cbor.AppendUint(enc, 1)
	enc = cbor.AppendUint(enc, 2)
	if _, err := cborbind.DecodeCBORInArrayFrom[PlayerInput](enc); err == nil {
		t.Fatal("a short array decoded")
	}
}

// The delegating pair is what a driver-generic consumer reaches.
func TestDriverInterfaceIsSatisfied(t *testing.T) {
	var a cbor.Appender = PlayerInput{Tick: 3}
	var d cbor.Decodable = &PlayerInput{}
	enc := a.AppendCBORTo(nil)
	if err := d.DecodeCBORFrom(enc); err != nil {
		t.Fatal(err)
	}
	if d.(*PlayerInput).Tick != 3 {
		t.Fatalf("got %+v", d)
	}
}

// The array shape is the smaller of the two, which is the trade it exists for.
func TestArrayIsSmallerThanMap(t *testing.T) {
	arr := cborbind.AppendCBORInArrayTo(nil, PlayerInput{Tick: 1, Btn: 2})
	mp := cborbind.AppendCBORInMapTo(nil, Snapshot{Round: 1})
	if len(arr) == 0 || len(mp) == 0 {
		t.Fatal("empty encoding")
	}
	if len(arr) >= len(cbor.AppendText(nil, "tick"))*3 {
		t.Logf("array %d bytes, map %d bytes", len(arr), len(mp))
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
		t.Fatalf("generated codecs do not round-trip: %v\n%s\n%s", err, output, code)
	}
}

// codecBody returns the emitted function starting at header, so an assertion
// about one codec cannot be satisfied by another.
func codecBody(t *testing.T, source, header string) string {
	t.Helper()
	i := strings.Index(source, header)
	if i < 0 {
		t.Fatalf("no %q in generated source", header)
	}
	rest := source[i:]
	if end := strings.Index(rest, "\n}\n"); end >= 0 {
		return rest[:end]
	}
	return rest
}
