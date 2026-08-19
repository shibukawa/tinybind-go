---
id: system:tinygodriver-cbor
type: system
title: tinygodriver CBOR Codec
---
Zero-allocation CBOR primitives and two named profiles, already shipped; the encoding layer a CBOR binding mode generates against rather than reimplements.

```yaml
package: github.com/shibukawa/tinygodriver/encoding/cbor
released_in: tinygodriver v1.2.5
provenance: moved into the driver from the downstream game framework, then given what a wire format needs; the driver catalog carries its own concepts for the move and its cost
reason_for_existing: a realtime message per player per tick cannot afford an allocating encoder, and no reflection-based CBOR library on the TinyGo path is allocation-free
what_it_already_solves:
  encode: AppendUint, AppendInt, AppendNegative, AppendBytes, AppendText, AppendBool, AppendNull, AppendFloat, AppendTag, AppendArrayHeader, AppendMapHeader, AppendRaw; each writes one item into a caller-owned buffer, with no limits, no validation and no error return
  decode: Reader over a byte slice, reusable through Reset, borrowing strings from the input
  width_enforcement: ReadInt8/16/32/64 and ReadUint8/16/32/64; a value too wide for the declared width is ErrIntegerOverflow rather than a silent truncation
  skip: Reader.Skip over an unknown item
  sub_item_capture: Reader.ReadRaw, returning a RawMessage borrowed from the input
  validation_without_decoding: Profile.Validate over a whole input, Profile.ValidateAppended over the region appended since a recorded length
  errors: '*Error carrying Offset and a container-route Path, wrapping one of ErrMalformed, ErrTruncated, ErrUnexpectedToken, ErrIntegerOverflow, ErrFloatRefused, ErrLimitExceeded, ErrProfileViolation, ErrDuplicateMapKey, ErrExtraneousData, ErrShortWrite'
  streaming_alternative: Encoder over an io.Writer and Decoder over an io.Reader, which allocate and are not the generated path
self_encoding_interfaces:
  Appender: 'AppendCBORTo(dst []byte) []byte; value receiver; appends exactly one complete item'
  Decodable: 'DecodeCBORFrom(data []byte) error; pointer receiver; data holds exactly one item and nothing after it'
  no_error_on_append: AppendCBORTo returns only the extended slice, so nothing below can report a refusal; the package doc names Profile.ValidateAppended as the check an encoder that cares should run over a foreign implementation's bytes
  named_Decodable_not_Decoder: cbor.Decoder is the streaming reader, so the decode-side interface could not take that name
profiles:
  Wire:
    for: fixed-shape realtime messages
    limits: 8 nested levels, 1024 container items, 4096-byte strings, 64 KiB input, 8 KiB raw message
    refuses: maps, tags, floats, indefinite lengths, text keys
  World:
    for: snapshots and episode logs
    limits: 32 nested levels, 65536 container items, 4 MiB strings, 64 MiB input and raw message
    admits: maps, tags, text keys, bytewise map key order
    refuses: floats, indefinite lengths
  both_refuse_floats: AllowingFloats reopens it, and a determinism-carrying schema never calls it
  key_order: BytewiseKeyOrder is RFC 8949 section 4.2.1 core deterministic encoding; LengthFirstKeyOrder is the CTAP2 order and is the zero value
measured_cost:
  source: package README, darwin/arm64, go test -bench . -benchtime 3s, over a fixed-shape wire message with a reused buffer and a reused Reader
  encode_append: 9.2 ns/op, 0 allocs
  decode_reader: 42.4 ns/op, 0 allocs
  wire_validate: 23.9 ns/op, 0 allocs
  world_validate: 90.0 ns/op, 0 allocs
  skip_unknown_field: 24.2 ns/op, 0 allocs
  contrast: the Encoder and Decoder paths cost 100.4 ns with 5 allocs and 168.0 ns with 7 allocs for the same message, which is why generated code names the Append and Reader layer and never the streaming one
  the_column_that_matters: allocations; a tick loop's steady state has to be free of them, and that is the budget generated code has to stay inside
the_reference_output_already_exists:
  where: encoding/cbor/codec_test.go
  what: a hand-written playerInput codec its own comment calls "the shape a generator would emit for the wire profile", plus a fixed64 standing in for a foreign fixed-point type
  pinned: TestWireMessageEncodesToPinnedBytes fixes the exact bytes and then re-validates them under the wire profile
  also_pinned: TestWireMessageRoundTrips, TestFixedShapeMessageIsZeroAllocationInSteadyState, TestAWideValueInANarrowFieldIsRefused
  therefore: an acceptance oracle rather than an illustration, per requirement:cbor-wire-codec
consumed_since: 2026-08-19, by decision:cborbind-runtime-package; go.mod requires tinygodriver v1.2.5
what_this_module_must_not_do:
  - reimplement the wire format, the profiles, or their enforcement
  - declare a second spelling of the codec interfaces, per rule:cbor-codec-interface-upstream
  - import the driver from anywhere but the runtime package of decision:cborbind-runtime-package
related:
  - decision:cborbind-runtime-package
  - rule:cbor-codec-interface-upstream
  - requirement:cbor-wire-codec
  - requirement:cbor-world-codec
  - system:tinybind
```
