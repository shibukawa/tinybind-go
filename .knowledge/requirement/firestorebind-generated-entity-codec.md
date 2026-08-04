---
id: requirement:firestorebind-generated-entity-codec
type: requirement
title: Generated Firestore Entity Codec
---
For each tagged type, emit an entity encoder, an entity decoder, a kind accessor, and a key builder into the user package, with compile-time assertions that they satisfy the runtime interfaces.

```yaml
status: implemented
proposed: 2026-08-03
implemented: 2026-08-04
built:
  analysis: generator/firestorebind.go and generator/firestorebind_types.go
  emitter: generator/firestorebind_emit.go and generator/firestorebind_decode.go
  wiring: generator/firestorebind_generate.go, writing firestorebind_gen.go
  fixture: internal/firestorefixture
relation_to_the_driver_mapper: system:tinygodriver-firestore gained MarshalEntity in v1.1.5, reading the datastore tag; this codec reads firestore and is the intended path, and a struct carrying both is a generation error per rule:firestore-tag-options
input:
  directive: "go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -dir ."
  declaration: a Go struct whose fields carry rule:firestore-tag-options tags
output_per_type:
  - "func (r Reading) EncodeEntity() datastore.Entity"
  - "func (r *Reading) DecodeEntity(e datastore.Entity) error"
  - "func (r Reading) Kind() string"
  - "func (r Reading) EntityKey() datastore.Key"; only when an identifier tag is present
  - "func (r Reading) EntityVersion() int64"; only when a version tag is present
  version_is_not_usage_directed: the runtime asks for it by interface assertion, which is not a call the generator can discover, so the tag alone emits it
  no_table_definition: there is nothing to emit; kinds are implicit, per decision:firestore-no-schema-artifact
assertions:
  - "var _ firestorebind.EntityEncoder = Reading{}"
  - "var _ firestorebind.EntityDecoder = (*Reading)(nil)"
  reason: a stale generated file becomes a build failure instead of a silently wrong codec
mapping: data:firestore-property-mapping
validation: rule:firestore-tag-options
key_handling: decision:firestore-key-identity
encode:
  properties: every tagged field that is not an identifier or a parent
  key: set on Entity.Key from EntityKey, or left incomplete when the identifier field is zero
  read_only_fields: Version, UpdateTime and CreateTime are never written; the driver ignores them on the way out and so does the encoder
  unindexed: a noindex tag wraps the value in datastore.Unindexed, which composes with every constructor
  omitempty: the property is absent from the map rather than set to Null, and the two differ to a filter
decode:
  properties: absent leaves the zero value; a present Null leaves the zero value or a nil pointer
  key_fields: filled from Entity.Key, so a decoded value carries its own identity
  version: an optional field tagged version receives Entity.Version, which is what feeds a later conditional write
  wrong_kind: a property whose stored kind does not match the field is a field-level error naming the property, the expected kind and the kind found
  extra_properties: ignored, because Datastore is schemaless and an older writer is normal rather than exceptional
number_types_do_not_merge:
  fact: Integer and Double are distinct wire types, unlike the single DynamoDB N
  effect: an int64 field decodes only from integerValue and a float64 field only from doubleValue; a value written by an earlier schema as the other one is a decode error, not a conversion
  why_not_coerce: Datastore sorts and compares the two separately, so a silent conversion would make a value the query that found it can no longer find
usage_direction:
  policy: rule:usage-directed-generation
  Store, StoreAll or a queued transaction write: EncodeEntity plus nested encoders
  Load, LoadAll, a declared query or a transaction read: DecodeEntity plus nested decoders
  key_builder: emitted from the tag, not from a discovered call, per decision:firestore-key-identity
  registration: none, matching decision:dynamobind-static-dispatch; no init entry is emitted
nested_types:
  encoding: a struct field becomes a nested entityValue, generated recursively
  no_key: an entityValue carries no key, so a nested type declaring an identifier tag is a generation error naming both types
  depth: datastore.MaxNestingDepth is the wire cap; a bound type whose static nesting already exceeds it is a generation error, and a recursive slice is a documented runtime failure
  inheritance: a nested type needs no tag of its own to be reachable, and its fields follow the same tag rules
runtime_ownership:
  - the generated file declares no helper type; shared code lives in firestorebind, per decision:generated-runtime-in-module
  - the generated file imports the driver for datastore.Entity, datastore.Key, datastore.Value and the value constructors
determinism: same input yields byte-identical output, as with every other generator mode
acceptance:
  - a multi-byte string, a blob, a nested entity, a timestamp and an empty string round trip
  - an int64 beyond float64 precision survives as text
  - an absent property and a Null property decode the same way and encode differently
  - a noindex field encodes with excludeFromIndexes set and decodes identically to an indexed one
  - a type whose identifier field is zero encodes an incomplete key and inserts
  - a decoded value's key fields match the key it was read by
related:
  - requirement:firestorebind-product-goals
  - data:firestore-property-mapping
  - rule:firestore-tag-options
  - decision:firestore-key-identity
  - decision:generated-runtime-in-module
  - rule:usage-directed-generation
  - requirement:dynamobind-generated-item-codec
  - concept:code-generation
```
