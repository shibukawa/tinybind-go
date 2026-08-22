# cborbind guide

`cborbind` generates CBOR codecs for the types you encode and decode. There is
no declaration to write and no codec to ask for: calling an entry point is the
ask.

```go
buf = cborbind.AppendCBORInArrayTo(buf[:0], input)
input, err := cborbind.DecodeCBORInArrayFrom[PlayerInput](buf)
```

The generator reads those calls, writes the codec, and puts the matching method
on the type. The first build after adding a call does not compile — the method
does not exist yet — so run the generator and build again. That is the same
bootstrap every other generated mode has, and it is the trade that makes a
missing codec a compile error instead of a run-time surprise.

- [The name says which container](#the-name-says-which-container)
- [Which one to pick](#which-one-to-pick)
- [What gets generated](#what-gets-generated)
- [Directions](#directions)
- [Both shapes on one type](#both-shapes-on-one-type)
- [Field types](#field-types)
- [There is no profile](#there-is-no-profile)
- [Generation](#generation)
- [Relation to the HTTP CBOR mode](#relation-to-the-http-cbor-mode)

## The name says which container

A struct encodes either as a CBOR array or as a CBOR map, and there are two
entry points per direction because those are two different contracts:

| | array | map |
| --- | --- | --- |
| on the wire | positional, no member names | text keys, RFC 8949 bytewise order |
| unknown member | cannot exist | skipped on decode |
| adding a field | both ends must be rebuilt together | old builds keep reading |
| size | smaller | carries the key names |

```go
cborbind.AppendCBORInArrayTo(dst, v)          // array
cborbind.AppendCBORInMapTo(dst, v)            // map
cborbind.DecodeCBORInArrayFrom[T](data)       // array
cborbind.DecodeCBORInMapFrom[T](data)         // map
```

Calling the map entry point on a type generated for the array shape does not
compile. The constraint is shape-specific on purpose: had both shapes shared
one method, the call would build and quietly produce the other shape's bytes.

## Which one to pick

One question decides it: **can both ends be updated at once?**

If yes — a game client and its server shipped together, two processes you
deploy as a unit — take the array. A three-field message with a nested struct
is six bytes.

If no — a stored snapshot, a queue payload, anything a older build may read —
take the map. It pays for the key names and buys the ability to add a field
without coordinating a release.

## What gets generated

For `AppendCBORInArrayTo(dst, PlayerInput{})` the generator writes:

```go
func appendPlayerInputCBORArray(dst []byte, v PlayerInput) []byte
func (v PlayerInput) AppendCBORInArrayTo(dst []byte) []byte
func (v PlayerInput) AppendCBORTo(dst []byte) []byte   // delegates
```

The third one is the driver's own `cbor.Appender`, so a consumer holding that
interface reaches your type without knowing about cborbind. The decode side
mirrors it with `DecodeCBORInArrayFrom` and `cbor.Decodable`.

Member names come from the `json` tag, the same names the JSON codec uses. One
struct spells its wire names once.

Nothing is registered and no `init` is emitted. Dispatch is resolved at
generation, so there is no lookup per message.

## Directions

The direction comes from which entry point you call. A package that only
appends gets no decoder, at the root and at every struct below it — which is
the point on a wasm client, where an unused decoder is bytes in the binary.

## Both shapes on one type

A type may have both, since the two methods do not collide:

```go
cborbind.AppendCBORInArrayTo(dst, t)   // to the peer that ships with you
cborbind.AppendCBORInMapTo(dst, t)     // to the log that outlives you
```

Such a type gets no delegating `AppendCBORTo`: there is no unambiguous shape to
delegate to, and leaving the ambiguity visible in the type beats resolving it by
declaration order.

## Field types

`string`, `bool`, `float64`, every fixed-width integer (`int8` through
`uint64`, plus `int` and `uint`), a nested struct, a slice of those, and a
`map[string]…`. A named type over any of them works and converts in both
directions.

Anything else is a generation error naming the type and the field. A
`payload:"*"` rest map, a field carrying its own JSON codec and no CBOR one,
and an uploaded file are each refused rather than silently dropped.

## There is no profile

No option, no CLI flag, no restriction struct. What a codec can encode is a
property of the generator; what it does encode is a property of your struct. If
you do not want floats in your protocol, do not declare a `float64` field —
that is the same statement, in the place that already carries it.

## Generation

```bash
tinybind-gen generate -dir .
```

A package calling no entry point emits no CBOR at all and links no CBOR
implementation. `cborbind` itself imports nothing: the interfaces are spelled in
byte slices, and only the generated code names the driver.

The generated codecs build and run under TinyGo for `wasm` and `wasip1`.

## Relation to the HTTP CBOR mode

`-cbor-http` is a different thing: it makes every route accept and answer
`application/cbor`, and it is a project-wide option rather than a per-type call.
The two can coexist. A type used by both gets one codec per keying, and they
agree about member names because both read the same `json` tags. See
[httpbind.md](httpbind.md) for the negotiation.
