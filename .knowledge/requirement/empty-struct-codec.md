---
id: requirement:empty-struct-codec
type: requirement
title: A Referenced Struct Always Gets A Codec, Or Generation Fails
---
Emit a codec for a struct that ends up with no mappable fields, and check before emission that every nested struct a codec calls into has one, so the generated file cannot reference a function that was never written.

```yaml
priority: must
source:
  - system:odjson harness 2026-09-12: Description{Urls []interface{}} got no plan, UserEntities' generated encoder and decoder called appendDescriptionJSON and decodeDescriptionJSON, and the DO NOT EDIT file did not compile
review_gate: proposed
status: implemented 2026-09-12; see as_built
as_built:
  plan: analyzeStruct returns a plan with no fields instead of ok=false, so the struct encodes as {} and decodes by consuming one object
  check: generator/emit.go checkNestedTypesEmitted, beside checkForeignFieldDirections, names parent, field and child when a nested struct has no codec in the emission
  walk: a type with no name to match discards the unread input, so the generated file compiles with an empty member switch
  test: generator/any_field_test.go
problem:
  where: generator/plan.go analyzeStruct returns ok=false when fields is empty, so the type is neither in plan.Types nor in unmappable; emitAppendValue and the decoder emitter write the child call for every KindStruct field unconditionally
  effect: a compile error inside generated source naming a function the generator chose not to write; the failure rule:generated-source-not-discovered already recorded once under another cause
  today_reachable_by: any struct whose every field is any, error, a foreign type without codec methods, or json:"-"; with requirement:any-field-kind the first cause goes away and the others remain
two_halves:
  emit_the_empty_codec: a struct with no fields encodes as {} and decodes by consuming one object and skipping every member; that is a valid codec and what encoding/json does, so analyzeStruct returns a plan with zero fields instead of ok=false
  check_before_emission: beside checkForeignFieldDirections, walk every KindStruct and slice-of-struct field of the types about to be emitted and require its TypeName in the emitted set; a miss is a generation error naming parent, field and child, never a compile error downstream
why_both: the codec fixes the case found; the check catches every variant not yet found, which is the same shape requirement:json-codec-interface took for foreign fields
test: a generator fixture with an empty nested struct compiles and round-trips {}; a fixture forcing the check to fire asserts the diagnostic text
related:
  - requirement:any-field-kind
  - requirement:json-codec-interface
  - concept:code-generation
```
