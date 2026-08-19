---
id: requirement:cbor-wire-codec
type: requirement
title: CBOR Wire Profile Codec
---
Emit a fixed-order, fixed-length array codec whose field widths are enforced on both sides and whose steady state allocates nothing, matching bytes the driver has already pinned.

```yaml
priority: must
status: implemented 2026-08-19
as_built:
  measured: the generated PlayerInput codec produces the driver's pinned bytes exactly, validates under cbor.Wire, round-trips bytewise, and allocates zero per run on both sides over 200 runs with a reused buffer and a reused Reader
  cross_target: the same message encodes to the same bytes on darwin/arm64 under Go and on wasip1 under TinyGo 0.41.1, asserted by testdata/cmd/tinygo-cbor-smoke rather than inferred
  refusals_verified: an array of 3 for a schema of 4, and a value beyond uint16 in a uint16 field
  emission: one sized append per field, one width-enforcing read per field, and nested planned structs appending into the same destination through a named function
  a_platform_width_integer_is_refused: int and uint are rejected outright, which the downstream requirements did not ask for; they are 64 bits on a host and 32 on wasm, so the two ends would disagree about what fits
  error_shape: a shape disagreement returns a driver *cbor.Error carrying the byte offset and the route, built inline; no fmt and no helper of this module's own reaches the generated file
  string_fields_allocate_once: ReadText copies, where every other read borrows or is a scalar; a string field is the one field kind whose decode is not allocation-free, and the wire profile's fixed-shape messages rarely hold one
  reachable_through: the Go API today, not the combined CLI; see decision:cborbind-runtime-package the_combined_cli_run_refuses_a_sized_integer
source:
  - downstream game framework CBOR requirements 2026-08-19, its section 4.2
  - system:tinygodriver-cbor codec_test.go, which is the reference output rather than an example
for: fixed-shape realtime messages, one per player per tick
encode:
  container: an array of the struct's field count; no field names, no map, no optional fields, no tags
  field_order: declaration order, and part of the protocol
  per_field: one sized append; a uint32 field writes through AppendUint
  nested_planned_struct: appends into the same destination at any depth, so composition costs no allocation and no intermediate buffer
decode:
  header: ReadArrayHeader, checked against the schema's field count and refused on a mismatch, indefinite length included
  per_field: one width-enforcing read; a uint32 field reads through ReadUint32, so a value too wide is ErrIntegerOverflow rather than a silent truncation
  no_skipping: an unknown field cannot exist under this profile, and skipping one would be pretending the two ends may disagree about the schema, which the framework's protocol-version rule refuses outright
zero_allocation_steady_state:
  both_sides: yes
  caller_owns: the destination buffer and the Reader, reused across messages
  gate_shape: the driver's TestFixedShapeMessageIsZeroAllocationInSteadyState, measured with AllocsPerRun
  budget: 9.2 ns to encode and 42.4 ns to decode the reference message, per system:tinygodriver-cbor measured_cost; the generated code has to stay inside it, which is the main constraint on how it is emitted
round_trip_is_bytewise:
  what: re-encoding what was decoded reproduces the same bytes
  why_it_matters: a replay compares digests, so this is a protocol property rather than a tidiness one
  already_pinned: the driver's TestWireMessageRoundTrips
the_acceptance_oracle_is_upstream:
  what: the generator emits a PlayerInput codec producing the bytes of TestWireMessageEncodesToPinnedBytes
  strong_form: that hand-written file can be replaced by generated output with no test change
  what_it_pins_beyond_bytes: the array-of-4 header, the shortest-form integer encodings, a foreign field reached through its own method, and a decode path with no runtime type switch
digest_equality_across_targets:
  what: the same message encodes to identical bytes on darwin/arm64, linux/amd64 and js/wasm
  why_it_is_this_requirement_s: the framework's determinism property is reached through generated code, so generated code is where it can be lost
  depends_on: rule:cbor-deterministic-types for the type set; a scaled field's own bytes are its type's, per decision:cbor-scale-lives-in-the-type
acceptance:
  - the generated PlayerInput codec produces the pinned bytes, and the hand-written reference file can be deleted
  - encode and decode of a fixed-shape message allocate zero in steady state, with a reused buffer and a reused Reader
  - an array header whose count disagrees with the schema is refused
  - a value too wide for its declared field width is refused rather than truncated
  - decode then encode reproduces the input bytes
  - generated code links and runs under TinyGo for js/wasm, per requirement:tinygo-wasm
  - the same message encodes identically on darwin/arm64, linux/amd64 and js/wasm
related:
  - system:tinygodriver-cbor
  - requirement:declared-cbor-codec
  - requirement:cbor-world-codec
  - requirement:cbor-composite-field-kinds
  - decision:cbor-scale-lives-in-the-type
  - rule:cbor-deterministic-types
  - requirement:tinygo-wasm
open_questions:
  - whether the generator emits the pinned-bytes and round-trip tests as well; concept:future-generators lists test generation as a future idea, and this is the case where the pinned bytes are the protocol, so an unpinned generated codec is a protocol with no record of what it used to be
```
