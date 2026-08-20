// Command tinygo-cbor-smoke exercises a generated CBOR codec on a TinyGo wasm
// target, which is where a client actually runs.
//
// It is a smoke test rather than a unit test: what it proves is that the
// generated file links under TinyGo for js/wasm and produces the same bytes it
// produces on the host. The determinism gate that makes that true is checked in
// generator/cborbind_test.go; this is the check that it survives a second
// compiler.
package main

import (
	"bytes"
	"fmt"

	"github.com/shibukawa/tinygodriver/encoding/cbor"
)

// Fixed1024 is a fixed-point value at 1/1024, carrying its own encoding. The
// scale is the type's, which is why a second scale would be a second type
// rather than a tag on the field.
type Fixed1024 int64

func (f Fixed1024) AppendCBORTo(dst []byte) []byte { return cbor.AppendInt(dst, int64(f)) }

func (f *Fixed1024) DecodeCBORFrom(data []byte) error {
	r := cbor.ReaderOver(data, cbor.DecoderOptions{})
	v, err := r.ReadInt()
	if err != nil {
		return err
	}
	if !r.Done() {
		return cbor.ErrExtraneousData
	}
	*f = Fixed1024(v)
	return nil
}

// PlayerInput is the reference wire message. Its bytes are pinned below and in
// the driver's own codec_test.go, and they must be the same on every target.
type PlayerInput struct {
	Tick    uint32
	MoveX   Fixed1024
	MoveY   Fixed1024
	Buttons uint16
}

// The generated codec is checked in beside this file, so the smoke test needs
// no generator run to build. Regenerate it from this directory with:
//
//	go generate ./testdata/cmd/tinygo-cbor-smoke
//
//go:generate go run generate.go

func main() {
	in := PlayerInput{Tick: 1234, MoveX: -1, MoveY: 0, Buttons: 3}
	want := []byte{0x84, 0x19, 0x04, 0xd2, 0x20, 0x00, 0x03}

	got := in.AppendCBORTo(nil)
	if !bytes.Equal(got, want) {
		panic(fmt.Sprintf("encoded %x, want %x", got, want))
	}
	// The wire profile, spelled out the way every consumer names its own subset
	// since driver v1.2.7.
	wireProfile := cbor.Profile{
		Name:             "wire",
		RejectMaps:       true,
		RejectTags:       true,
		RejectFloats:     true,
		RejectIndefinite: true,
		RejectTextKeys:   true,
	}
	if err := wireProfile.Validate(got, cbor.DecoderOptions{}); err != nil {
		panic(err)
	}

	var out PlayerInput
	if err := out.DecodeCBORFrom(got); err != nil {
		panic(err)
	}
	if out != in {
		panic(fmt.Sprintf("round trip gave %+v, want %+v", out, in))
	}
	checkDelta()
	fmt.Print(CBORProtocolVersion)
}

// House and City are the delta half of the smoke: a two-level identified
// hierarchy, so the generated diff, apply, insertion sort and the clear over
// the scratch index all have to link under TinyGo too.
type House struct {
	ID    uint32 `cbor:"id,identity"`
	Power int32
}

type City struct {
	ID     uint32 `cbor:"id,identity"`
	Houses []House
}

type World struct {
	Tick   uint32
	Cities []City
}

func smokeWorld() World {
	return World{Tick: 10, Cities: []City{
		{ID: 1, Houses: []House{{ID: 10, Power: 5}, {ID: 11, Power: 6}}},
		{ID: 2, Houses: []House{{ID: 20, Power: 7}}},
	}}
}

// checkDelta diffs one changed field, sends the delta through its own codec,
// and applies it to a fresh baseline. What it proves is that the delta path
// links and agrees with the host, not that the diff is correct -- that is
// generator/cborbind_test.go's job.
func checkDelta() {
	base, cur := smokeWorld(), smokeWorld()
	cur.Cities[0].Houses[1].Power = -1000

	encoded := DiffWorld(base, cur).AppendCBORTo(nil)
	// World bit 1 is Cities; City bit 0 is Houses, its identity taking no bit;
	// House bit 0 is Power. Seventeen bytes to reach one int32 two levels down.
	want := []byte{0x82, 0x02, 0x82, 0x04, 0x82, 0x01, 0x82, 0x01, 0x82, 0x04, 0x82, 0x0b, 0x82, 0x01, 0x39, 0x03, 0xe7}
	if !bytes.Equal(encoded, want) {
		panic(fmt.Sprintf("delta encoded %x, want %x", encoded, want))
	}

	var back WorldDelta
	if err := back.DecodeCBORFrom(encoded); err != nil {
		panic(err)
	}
	got := smokeWorld()
	if err := ApplyWorldDelta(&got, back); err != nil {
		panic(err)
	}
	if !bytes.Equal(got.AppendCBORTo(nil), cur.AppendCBORTo(nil)) {
		panic("applying the delta did not reproduce the sender's bytes")
	}
}
