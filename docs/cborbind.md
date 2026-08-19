# cborbind

Generated CBOR codecs for a realtime protocol: a fixed-shape wire format for
messages sent per tick, and an evolvable world format for snapshots.

The encoding itself is not this module's. `tinygodriver/encoding/cbor` (v1.2.5
and later) already carries the append primitives, the reusable `Reader`, the
width-enforcing reads, the two named profiles and the error type. cborbind adds
the declaration that asks for a codec, and the generator that writes one.

Everything below assumes `github.com/shibukawa/tinygodriver v1.2.5` or later.

## Declaring a codec

Generation is declaration-driven. A game hands its message types to a session
framework which encodes them generically, so there is no generic call in the
game's own source for usage-directed generation to find. Write the declaration
at package level, beside the type:

```go
var _ = cborbind.GenerateWireCodec[PlayerInput]()
var _ = cborbind.GenerateWorldCodec[WorldState]()
```

The call runs at `init` and does nothing. The declaration is the point.

Six annotations exist, one per profile and direction:

| Annotation | Emits |
|---|---|
| `GenerateWireCodec[T]` | wire encoder and decoder, and both methods |
| `GenerateWireEncoder[T]` | wire encoder and `AppendCBORTo` |
| `GenerateWireDecoder[T]` | wire decoder and `DecodeCBORFrom` |
| `GenerateWorldCodec[T]` | world encoder and decoder, and both methods |
| `GenerateWorldEncoder[T]` | world encoder and `AppendCBORTo` |
| `GenerateWorldDecoder[T]` | world decoder and `DecodeCBORFrom` |

Naming a direction is worth doing: emitting a decoder for a message the client
only ever sends is code size in a wasm binary.

The declaration must live in the package that declares the type. Naming a
foreign type is a generation error, not silence.

## The profile is part of the contract

The two ends of a connection must not disagree about which profile is in use,
so the annotation names it and the generated code pins it. A wire codec will
not read a world message.

**Wire** is frozen. A struct encodes as a fixed-length array in declaration
order — no field names, no optional fields, no room for a field the schema does
not know. Field order *is* the protocol.

**World** is evolvable. A struct encodes as a map with deterministic key order,
and its decoder skips a key it does not know, which is what lets a snapshot
outlive the version that wrote it.

One type cannot carry both: two profiles would mean two `AppendCBORTo` methods.
Declare a separate type per profile.

## The codec interface is the driver's

cborbind declares no interface of its own. The contract is
`cbor.Appender`/`AppendCBORTo` and `cbor.Decodable`/`DecodeCBORFrom`, already
declared in the driver. Two spellings of one contract would mean a type could
satisfy the wrong one and be skipped without a word.

The generator recognizes both structurally, so a package need not import
cborbind merely to have its types admitted.

**A type carrying the methods wins.** If `T` already has `AppendCBORTo`, every
field of type `T` is encoded through it — at any depth, and inside a slice.
Generating a codec for a type whose author wrote an encoder, and then using the
generated one, would silently produce bytes the author did not intend.

Declaring a codec for a type that *already* has the methods is a generation
error rather than a second codec: remove one or the other.

## Fixed point: the scale is the type's

`cbor.Appender` is a method on a type, and a scale is a property of a field. So
declare one type per scale and let each carry its own conversion:

```go
// Fixed1024 is a fixed-point value at 1/1024.
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
```

A position at 1/1024 and a velocity at 1/65536 are then two types, and the
generator never sees a scale, never converts one, and imports no fixed-point
library. The type name is the evidence: two fields of one declared type carry
one scale by construction, which a tag could not guarantee.

## What is refused, and why

Every rejection below is a value that would encode, round trip, and then differ
between two runs or two targets. The check covers the whole type set reachable
from a declaration; nothing marks that set, reachability is the definition.

| Refused | Reason |
|---|---|
| `float32`, `float64` | rounding varies with the target, and with fused multiply-add |
| `int`, `uint` | 64 bits on a host target and 32 on wasm, so the two ends disagree about what fits |
| `map` | Go randomizes iteration order, so traversal and output vary per run |
| `interface`, pointer | identity and aliasing are not part of the bytes |
| `time.Time` | not a function of the tick |
| recursive type | no fixed shape to generate a codec for |
| anonymous struct | no name to generate a codec for |

A float behind a named type is refused too, and the diagnostic names the
underlying kind — the declared name looks innocent, and a message naming only
it sends you looking in the wrong place.

Use `int8`/`16`/`32`/`64` and `uint8`/`16`/`32`/`64` for numbers, an ordered
slice where a map is tempting, and a fixed-point type for anything fractional.

This check is a build gate, not a lint, and is not among the features
`DisableFeatures` can turn off.

## Tags

```go
type Entity struct {
	ID   uint32 `cbor:"id,key=1"`
	PosX Fixed1024 `cbor:"x,key=2"`
	Skip string `cbor:"-"`
}
```

`-` drops a field under both profiles. The name and `key=N` apply to the world
profile's map keys, and are inert under wire, where a field is identified by
its position. An unknown option is a generation error rather than a silently
inert tag.

`key=N` is all or nothing: number every field or none. A map half keyed by
integers and half by text is legal CBOR and an unreadable schema. With a full
numbering the map uses integer keys, which are smaller.

Keys are emitted in the bytewise order of their encoded form — the core
deterministic encoding of RFC 8949 — computed at generation time, so two runs
write the same bytes.

## What the generated code looks like

```go
func appendPlayerInputCBORWire(dst []byte, v PlayerInput) []byte {
	dst = cbor.AppendArrayHeader(dst, 4)
	dst = cbor.AppendUint(dst, uint64(v.Tick))
	dst = v.MoveX.AppendCBORTo(dst)
	dst = v.MoveY.AppendCBORTo(dst)
	dst = cbor.AppendUint(dst, uint64(v.Buttons))
	return dst
}

func (v PlayerInput) AppendCBORTo(dst []byte) []byte { return appendPlayerInputCBORWire(dst, v) }
```

Nothing is registered and no `init` function is emitted. Dispatch is resolved
at generation: the emitted call names one path, with no type switch and no
assertion. A registry lookup per message, at tick rate per player, would be a
visible fraction of a 9.2 ns encode.

A type reached only as a nested field gets the functions and no public method —
a method is code size in every binary carrying the type, and nothing asked for
one.

## Cost

Steady state is allocation-free on both sides when the caller owns and reuses
the destination buffer and the `Reader`:

```go
buf = in.AppendCBORTo(buf[:0])

r.Reset(encoded)
err := decodePlayerInputCBORWire(&r, &out)
```

Two exceptions, both deliberate:

- A `string` field allocates once on decode. `ReadText` copies where every other
  read borrows.
- A world-profile decode zeroes its target first, so an omitted key means zero
  rather than "whatever was there last time". That costs the slice capacity a
  wire decode reuses. A snapshot is not the tick loop.

A `[]byte` field is copied into the field's own capacity rather than borrowed,
so the value outlives the buffer it was read from without allocating in steady
state.

## Protocol version

Every generated file carries two constants:

```go
const CBORProtocolVersion = "03119a9fb229a399"
const CBORSchema = `type PlayerInput wire
  uint32
  self Fixed1024
  self Fixed1024
  uint16
`
```

`CBORProtocolVersion` is a digest of `CBORSchema`, which covers wire-observable
shape and nothing else: profile, field order, wire key, kind and width per
field, and the name of each self-encoding type. It does not cover the generator,
`go.mod`, or which directions you asked for — regenerating with a newer tinybind
that emits the same bytes leaves it unchanged, so a generator upgrade is not a
coordinated redeploy of both ends. Any change that moves one byte moves it.

This is a different digest from the input hash that decides whether to
regenerate, which legitimately covers things the wire never sees.

The schema is emitted, not just the digest, so a version mismatch is diagnosed
by diffing two schemas rather than by observing that two opaque numbers differ.

## Running the generator

```go
g := &generator.Generator{Options: generator.DefaultOptions()}
path, err := g.GenerateCborCodecs(dir, "", "")
```

The output file is `cborbind_gen.go`.

`tinybind-gen generate` runs this pass too, and writes the same file.

A struct carrying a CBOR codec needs no JSON codec, and does not get one: the
mapping emitter is usage-gated, so a type nothing binds, writes or decodes as
JSON produces no JSON output at all. The mapping pass does still *walk* every
package-level struct, and it maps only `string`, `int`, `int64`, `bool` and
`float64` — but a field kind it cannot map is now reported only for a struct
something asks it to map, so a message type of sized integers passes through it
untouched.

If you do ask for a JSON codec on such a type — a call site naming it, a
JSON-mapped struct holding it as a field, or `-generate-all` — the refusal is
the one it always was, naming the type and the field.

Features `cbor-wire-codec` and `cbor-world-codec` turn one profile off:

```go
options.DisableFeatures = []generator.Feature{generator.FeatureCBORWorldCodec}
```

## TinyGo

Generated codecs build and run under TinyGo 0.41.1 for `wasm` and `wasip1`, and
produce the same bytes there as on the host — which is what the determinism
rules above are for. `scripts/tinygo-check.sh` asserts the pinned bytes on both.

The dependency graph of a CBOR-only package holds no `net/http` and no
`database/sql`.

## Deltas

`GenerateWorldDelta[T]()` (or `GenerateWireDelta[T]()`) asks for a diff and an
apply on top of the codec. It implies the codec for that profile, so one
declaration is enough:

```go
var _ = cborbind.GenerateWorldDelta[World]()
```

What you get, for `World` and every struct it reaches:

```go
type WorldDelta struct { Present uint64; ... }

func DiffWorld(baseline, current World) WorldDelta
func DiffWorldInto(d *WorldDelta, baseline, current World)
func ApplyWorldDelta(v *World, d WorldDelta) error

func (v WorldDelta) AppendCBORTo(dst []byte) []byte
func (v *WorldDelta) DecodeCBORFrom(data []byte) error
```

`Present` names the fields carried: bit *n* is set when field *n* changed, and
every other field of the delta holds nothing. Both ends already agree on
`CBORProtocolVersion`, so a field is named by its bit rather than by a path.

### Identify your entities

A collection is diffed element by element only when its element type says which
field names it:

```go
type House struct {
	ID    uint32 `cbor:"id,identity"`
	Power int32
}
```

Without an identity nothing distinguishes an entity that changed from one that
was replaced, so the collection is carried whole under a single bit. That is
legal — and for a short fixed slice it is cheaper than diffing — but for an
entity list it produces deltas the size of a snapshot. Every collection carried
whole is listed in a comment at the top of the generated file.

An identity must be an integer or a string, and a type may declare one.

### What it costs

For a `world → city → house` hierarchy, changing one `int32` on one house:

```
17 bytes: 82028204820182028204820b82013903e7
```

Three of those are the value. The rest is the mask and identity chain that
addresses it. Each hierarchy level costs a mask, an identity and two array
heads, whatever the collection holds.

`DiffWorldInto` allocates nothing in the steady state once its delta has been
used a first time — retain the delta across ticks rather than calling
`DiffWorld`.

### What apply guarantees

`ApplyWorldDelta(&baseline, d)` reproduces the bytes the sender's current state
encodes to. It reproduces the *bytes*, not the Go value: CBOR carries no
difference between a nil slice and an empty one, so a collection a delta empties
comes back as an empty slice where the sender held nil.

Applying a delta to a value that is not the baseline it was diffed against
returns an error rather than half-applying. Identified collections come out in
identity order, so a receiver fed deltas and a sender holding the same entities
encode identically — which is what a replay comparing digests needs.

Order is therefore not state for an identified collection. If order means
something in your game, carry it in a field.
