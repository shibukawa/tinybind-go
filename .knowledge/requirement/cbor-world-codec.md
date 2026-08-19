---
id: requirement:cbor-world-codec
type: requirement
title: CBOR World Profile Codec
---
Emit a map codec with deterministic key order that tolerates an unknown field, since a snapshot format has to outlive the schema that wrote it.

```yaml
priority: must
status: implemented 2026-08-19
as_built:
  container: a map whose pairs are emitted in the bytewise order of the encoded key, computed at generation time so the plan, the schema description and the bytes all agree
  keys: text by default, taken from the Go field name or a cbor tag; integers when every field declares a key option
  numbering_is_all_or_nothing: a type numbering some fields and not others is a generation error, because a map half keyed by integers and half by text is legal CBOR and an unreadable schema
  skip: the decoder's default arm calls Reader.Skip, at 24.2 ns and no allocation
  verified: a slice of entities holding a type with its own codec round-trips, validates under cbor.World, re-encodes to the same bytes, and decodes with an extra unknown pair present
pending_change_for_the_delta:
  raised: 2026-08-19, by rule:cbor-entity-identity
  what: an identified collection must be emitted in identity order, not in slice order
  why: a delta keyed by identity cannot express a reordering, so a state reached by applying a delta and the same state encoded from a snapshot have to produce the same bytes
  cost: a sort per identified collection per encode, on the snapshot path rather than the tick path
  the_caveat_to_state: a decode then encode still reproduces the bytes, but a decode no longer reproduces the Go value's slice order for an identified collection; that order stops being state, which rule:cbor-entity-identity says outright
  not_built: the world codec emits slice order today, since nothing declares an identity yet
optional_field_semantics_settled:
  decided: 2026-08-19, while building, closing the question this concept opened
  what: an omitted key leaves that field zero, because the decoder zeroes the target before reading the map
  why_zero_rather_than_unchanged: a decoder that left a field alone would let a reused value inherit a field from the message before it, which is a wrong snapshot that nothing reports
  cost_accepted: zeroing loses the slice capacity a wire-profile decode reuses; a snapshot is not the tick loop, so the trade goes the other way here
  omitempty_not_reused: no tag vocabulary was carried over from JSON, since the question there is emptiness and the question here is presence
source:
  - downstream game framework CBOR requirements 2026-08-19, its section 4.3
for: snapshots and episode logs, which are written far less often than a tick message and read by a version that may not match
encode:
  container: a map
  key_order: deterministic and bytewise, which is the order the driver's World profile enforces
  keys: prefer integers over text where a stable field numbering exists; the profile permits both and integers are smaller
  admits: optional fields and tags, which the wire profile refuses
  never_from_a_go_map: the ordered traversal is generated, per rule:cbor-deterministic-types, because ranging a Go map is the nondeterminism that rule exists to remove
decode:
  unknown_field: skipped through Reader.Skip, at 24.2 ns and no allocation
  why_not_refused: schema tolerance is the reason this profile exists, and it is the exact opposite of requirement:cbor-wire-codec no_skipping; the same message under the two profiles must therefore behave differently, which is why decision:cborbind-runtime-package pins the profile into the generated code
optional_field_semantics:
  must_be_stated_not_inherited: the omitempty and omitzero reading of concept:standalone-json-codec is an encoding/json/v2 alignment decision with no meaning here
  the_actual_question: an omitted world-profile field means unchanged or absent, which is a different question from empty
  interacts_with: requirement:cbor-state-delta-generation, where unchanged is precisely what a delta encodes
  open: whether the existing tag vocabulary is reused, given a different meaning, or refused here as it is for a foreign JSON field
acceptance:
  - a world-profile message with an unknown field decodes, skipping it
  - the same message under the wire profile is refused
  - map keys are emitted in bytewise order, and that order does not vary between runs
  - a struct whose fields carry a stable numbering emits integer keys
related:
  - system:tinygodriver-cbor
  - requirement:declared-cbor-codec
  - requirement:cbor-wire-codec
  - rule:cbor-deterministic-types
  - requirement:cbor-state-delta-generation
  - concept:standalone-json-codec
open_questions:
  - what an omitted field means, and whether omitempty and omitzero are reused or refused
```
