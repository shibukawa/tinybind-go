---
id: requirement:nested-collections-and-pointers
type: requirement
title: Pointer Fields And Collections Of Collections
---
Map a pointer field as null-or-pointee and a collection whose element is itself a collection or a pointer, so a tree, a nullable object, a [][]float64 and a map of slices decode and encode like every other field instead of leaving the plan in silence.

```yaml
priority: must
source:
  - the jsonbench datasets (go-json-experiment/jsonbench) run through the generator 2026-09-12: golang_source's tree and twitter_status's retweeted_status are pointers, canada_geometry's coordinates a [][][2]float64, citm_catalog's topicSubTopics a map of slices, synthea's telecom a slice of maps; every one was dropped without a word, and string_unicode's "Box Drawing" name was split at the space into a duplicate case
review_gate: proposed
status: implemented 2026-09-12; see as_built
as_built:
  plan: KindPtr, and FieldPlan.Elem carrying the element's own plan when it is composite; StructTypeName walks the chain so usage propagation and the emitted-set check still find the struct at the end of it
  decode: generator/emit_read.go emitReadInto, one recursive value reader that decodes in place -- a pointee into new(T) when nil, a slice element into the slot append made, an array cell in place, a map value into a local -- with locals numbered by depth
  encode: emitAppendValue's pointer arm writes null or the pointee; nested loop variables are numbered by how deeply the source is indexed
  tags: tagLookup, lookupTag and tagPresent read through reflect.StructTag, so a quoted value may hold spaces
  refusal: a shape nothing recognises -- a channel, a function, a map with a non-string key, an array or map of one of those -- is an error naming the field; a foreign type without a codec stays a silent drop, as generator/foreign_field_test.go requires
  openapi: a pointer is its pointee's schema marked nullable; a nested collection recurses through compositeSchema
  cbor: refused by name, with omitempty, default and enum treating the new kind as composite
  slice_capacity: a slice's first allocation is sized from an estimate of its element, 8 for a small element down to 1 for a wide struct, which took a document of one-element slices of wide structs from a third more bytes than json/v2 to fewer
  tests: generator/nested_collection_test.go, shape assertions and a compiled round trip against encoding/json
related:
  - requirement:any-field-kind
  - requirement:empty-struct-codec
  - system:odjson
```
