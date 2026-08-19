---
id: requirement:cbor-delta-codec
type: requirement
title: The Delta Type Carries Its Own Codec
---
Generate the delta type's encoder and decoder with the delta type, so a caller hands the transport a value rather than assembling a document.

```yaml
priority: should
status: implemented 2026-08-19
as_built:
  root: 'Diff<T>, Diff<T>Into, Apply<T>Delta, and AppendCBORTo and DecodeCBORFrom on <T>Delta'
  nested: 'a <T>Delta per struct in the reachable set, a <E>ListDelta and an <E>Patch per identified element type'
  scratch_is_unexported: the collection delta holds an index map the diff reuses; it is never encoded and never ranged
source:
  - maintainer ask 2026-08-19, that the delta's encoder and decoder be generated ahead of time so the consuming side has nothing to write
what_is_emitted:
  type: 'a struct per world type, WorldStateDelta for WorldState, holding a presence mask and one field per encodable field of the source'
  methods: AppendCBORTo and DecodeCBORFrom on it, satisfying the driver's interfaces exactly as a message codec does
  therefore: a delta is passed to anything that takes a cbor.Appender, with no second surface to learn
why_a_named_type_and_not_a_generic_document:
  a_generic_document_would_be: a tree of any, built per tick, allocating per node, and type-asserted on the receiving side
  the_named_type_is: a struct the caller can retain and reuse across ticks, which is what makes the steady state of requirement:cbor-state-delta-generation allocation-free
  and_it_type_checks: applying a WorldStateDelta to an EntityState does not compile, where a generic document would fail at run time on a value that already crossed the network
the_delta_type_is_not_hand_written:
  reason: its field set, its mask width and its collection sub-deltas are all derived from the source type, and a hand-written one would drift from the schema the version pins
  consequence: it carries the DO NOT EDIT header with everything else generated
shape:
  mask: an unsigned integer field whose bits follow the source type's declaration order
  scalar_field: the source type's own field type, meaningful only when its bit is set
  nested_struct_field: that struct's delta type, so the nesting is mirrored
  identified_collection_field: a collection delta type of its own, holding its mask and the set, removed and patched groups of data:cbor-state-delta; it is a named generated type too, so it is retained and reused like the delta that holds it
  unidentified_collection_field: the source slice type, carried whole
profile:
  pinned_like_any_other_codec: a delta type generated for one profile reads only that profile's bytes, per requirement:declared-cbor-codec
  but_the_shape_is_the_same_under_both: data:cbor-state-delta is arrays and integers, so nothing about the delta forces a profile; the declaration picks one
declaration:
  form: 'the delta is asked for beside the codec, as var _ = cborbind.GenerateWorldDelta[WorldState]()'
  implies_the_codec: a delta is diffed from values that must also be encodable, so declaring a delta declares the world codec too rather than failing on its absence
  directions: not narrowed; a delta is sent by one side and read by the other, and the type declaring it is generally on the sending side while the same generated file serves the receiving one
acceptance:
  - a generated delta type satisfies cbor.Appender and cbor.Decodable
  - a delta round trips through its own codec and applies to the same result on the far side
  - a delta declaration with no codec declaration generates both
  - a delta type is not emitted for a type nothing declared one for
related:
  - requirement:cbor-state-delta-generation
  - data:cbor-state-delta
  - rule:cbor-entity-identity
  - requirement:declared-cbor-codec
  - rule:cbor-codec-interface-upstream
  - requirement:cbor-world-codec
```
