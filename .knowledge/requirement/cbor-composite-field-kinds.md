---
id: requirement:cbor-composite-field-kinds
type: requirement
title: CBOR Composite And Foreign Field Kinds
---
Admit a foreign type as a slice or map element, closing the gap fieldTypeKind still has, because a world state is entities in slices and the scalar inside them is the foreign type.

```yaml
priority: must
status: implemented 2026-08-19, for CBOR only
as_built:
  where: generator/cborbind_types.go resolve, recursing through slice elements
  admitted: a slice of a type carrying its own codec, a slice of a planned struct, a slice of a scalar, and any of them at depth
  decode_reuses_capacity: a slice is re-sliced when the existing capacity fits and allocated only when it does not, so a value decoded into repeatedly stops allocating after the first message
  byte_slices_are_copied_not_borrowed: ReadBytes borrows from the input, so the value is appended into the field's own capacity; borrowing would alias a buffer the caller is about to reuse
  verified: a WorldState holding a slice of entities whose fields are a fixed-point type with its own methods round-trips whole
the_json_gap_is_still_open:
  what_this_concept_first_claimed: that the fix would arrive through fieldTypeKind and the JSON side would inherit it
  what_was_actually_built: a separate go/types collector, the shape dynamobind and firestorebind already use, because interface satisfaction and underlying kinds have to be read from types rather than from syntax
  therefore: requirement:json-codec-interface not_built_yet.collections is untouched; a JSON slice of a foreign type is still dropped
  which_is_the_honest_trade: the CBOR mode needed a type-driven analysis for four other reasons, and routing it through the AST path to fix JSON on the way would have been the larger change, not the smaller one
source:
  - downstream game framework CBOR requirements 2026-08-19, its section 4.4
  - requirement:json-codec-interface not_built_yet.collections, which is the same gap seen from the JSON side
the_known_gap:
  recorded: requirement:json-codec-interface not_built_yet, as "a slice or map whose element is a foreign type is still dropped, since fieldTypeKind admits only scalars and planned structs as elements"
  for_json: a missing nicety, since the affected shapes are uncommon and a dropped field shows up as a missing member in a document someone reads
  for_cbor: blocking, because the framework's world state is entities in slices and the scalar inside them is exactly the foreign fixed-point type
  worst_available_failure: a silently dropped field leaves both ends agreeing about everything they did encode, so the protocol runs and the state diverges
required:
  - a slice whose element type carries the driver's codec interfaces is admitted and encoded element by element
  - a slice of a planned struct is admitted at any depth
  - a map is admitted only where rule:cbor-deterministic-types permits it, which is nowhere reachable from a world-state root
  - a field the generator cannot map is a generation error naming the type, the field and what it is underneath, never a drop
composition_at_depth:
  encode: a nested planned struct appends into the same destination, which composes at any depth with no allocation, exactly as the JSON append side already does
  foreign_encode: the field's own AppendCBORTo into the same destination
  foreign_decode: Reader.ReadRaw, which borrows a sub-item from the input rather than copying it, then the field's DecodeCBORFrom over that slice
  no_second_scan_of_the_whole: ReadRaw yields the sub-item the reader was going to walk anyway, so the cost is the sub-item and not the message
  resolved_at_generation: the emitted call names one path per field, with no runtime type switch, which is what the driver's reference codec writes by hand
direction_check_carries_over:
  what: a parent needing to encode requires AppendCBORTo on the field type, and a parent needing to decode requires DecodeCBORFrom
  when: where usage is settled, not at admission, since analyzeStruct runs before the parent's direction is known
  why_a_diagnostic: without it the emitted file names a method the type lacks, and that compile error lands inside a DO NOT EDIT file, which rule:named-type-field-kind already recorded once as the failure shape to avoid
acceptance:
  - a struct holding a slice of entities whose fields include a foreign fixed-point type generates a complete codec, with no field silently dropped
  - a foreign field at depth decodes through a borrowed sub-item
  - a foreign field whose type lacks the half its parent needs is a generation error naming the field and the missing method
related:
  - requirement:json-codec-interface
  - rule:cbor-codec-interface-upstream
  - requirement:cbor-wire-codec
  - decision:cbor-scale-lives-in-the-type
  - rule:cbor-deterministic-types
  - rule:named-type-field-kind
```
