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

func TestShapesUseTheirWireHeaders(t *testing.T) {
	array := cborbind.AppendCBORInArrayTo(nil, PlayerInput{Tick: 1})
	cr := cbor.ReaderOver(array, cbor.DecoderOptions{})
	n, indefinite, err := cr.ReadArrayHeader()
	if err != nil || indefinite || n != 3 {
		t.Fatalf("array header %d indefinite=%v err=%v", n, indefinite, err)
	}

	mapped := cborbind.AppendCBORInMapTo(nil, Snapshot{Round: 2})
	cr = cbor.ReaderOver(mapped, cbor.DecoderOptions{})
	pairs, indefinite, err := cr.ReadMapHeader()
	if err != nil || indefinite || pairs != 2 {
		t.Fatalf("map header %d indefinite=%v err=%v", pairs, indefinite, err)
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
