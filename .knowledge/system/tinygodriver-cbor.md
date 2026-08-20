---
id: system:tinygodriver-cbor
type: system
title: tinygodriver CBOR Codec
---
Zero-allocation CBOR primitives plus a format-restriction Profile any consumer builds as a struct literal; the encoding layer a CBOR binding mode generates against rather than reimplements.

```yaml
package: github.com/shibukawa/tinygodriver/encoding/cbor
released_in: tinygodriver v1.2.5; profile shape reshaped in v1.2.7
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
profiles_reshaped_v1_2_7:
  what: a Profile is now a format restriction only; Name, RequireSortedKeys, KeyOrder, and Reject fields for maps, tags, floats, indefinite lengths and text keys
  zero_value: restricts nothing; a consumer names its subset as a struct literal, needing nothing from the package
  limits_moved_out: every resource limit lives in DecoderOptions, chosen per deployment and passed alongside; Validate(data, opts), NewReader(data, opts), ReaderOver(data, opts)
  why_split: a profile belongs to the protocol and a limit to the deployment; bundling them made a deployment decision look like a protocol change, per the driver catalog's .knowledge/requirement/cbor-encoding-profiles.md
  wire_and_world_removed: the presets were one application's subsets; they are struct literals in the game server's own project now, and the driver's tests carry them as the worked example
  presets_remaining: Canonical, CTAP2 and COSE, length-first keys; Deterministic, RFC 8949 4.2.1, bytewise keys; both permit floats
  floats_are_ordinary: RejectFloats is opt-in on both Profile and DecoderOptions; a determinism-carrying schema switches it on, everyone else keeps floats
  nesting_is_a_safety_net: the default nesting bound is a stack guard far past any schema, not a per-profile budget
  key_order: BytewiseKeyOrder is RFC 8949 section 4.2.1 core deterministic encoding; LengthFirstKeyOrder is the CTAP2 order and is the zero value
the_v1_2_7_bump_and_what_followed:
  bumped: 2026-08-20; go.mod requires v1.2.7
  first_migration: the codec emitter briefly wrote the wire and world literals and their old ceilings into generated files, which put one application's profiles into a shared generator -- the exact overreach the driver's reshape had just named
  then: the maintainer removed the codec pass the same day rather than keeping the profiles under any spelling, per decision:cbor-codecs-are-application-side
measured_cost:
  source: package README, darwin/arm64, go test -bench . -benchtime 3s, over a fixed-shape wire message with a reused buffer and a reused Reader
  encode_append: 9.2 ns/op, 0 allocs
  decode_reader: 42.4 ns/op, 0 allocs
  skip_unknown_field: 24.2 ns/op, 0 allocs
  contrast: the Encoder and Decoder paths cost 100.4 ns with 5 allocs and 168.0 ns with 7 allocs for the same message, which is why generated code names the Append and Reader layer and never the streaming one
consumed_by: the generated HTTP codecs of requirement:cbor-http-body, from emitted user-package code only; go.mod requires tinygodriver v1.2.7
what_this_module_must_not_do:
  - reimplement the wire format, the profile mechanism, or their enforcement
  - declare its own spelling of the driver's codec interfaces
  - import the driver from any runtime package; only generated code names it
related:
  - decision:cbor-codecs-are-application-side
  - requirement:cbor-http-body
  - system:tinybind
```
