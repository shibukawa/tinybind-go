---
id: requirement:firestorebind-product-goals
type: requirement
title: firestorebind Product Goals
---
Generate Firestore Datastore entity codecs and key builders from one Go struct declaration, so no call site builds a datastore.Value or names a property string.

```yaml
status: partially implemented
proposed: 2026-08-03
implemented: 2026-08-04 for stages 1 and 3 below; stage 2 is not started
source: user request 2026-08-03, after system:tinygodriver-firestore shipped in tinygodriver v1.1.4
target: system:tinygodriver-firestore, at v1.1.5
shape_source: requirement:dynamobind-product-goals, whose staged plan is reused; what does not transfer is concept:dynamobind-firestorebind-mapping
problems:
  two_tag_paths:
    statement: the driver's own mapper reads a different tag than the generated codec does
    detail: v1.1.5 added MarshalEntity and UnmarshalEntity behind the datastore tag, so a struct can carry datastore and firestore tags that look interchangeable and disagree on every renamed property
    failure: silent; both paths compile and both produce an Entity, and only one matches what a query filters on
    check: a field carrying datastore but not firestore is a generation error, per rule:firestore-tag-options, which is what the driver's own doc comment asks a generator over it to do
    superseded_argument: this requirement was first written against v1.1.4, where the driver had no mapper at all and generating was the only way a struct reached it; that absence is gone and the drift argument carries the case on its own
  drift:
    statement: nothing connects a struct to its kind or its key
    detail: the kind string in NewQuery, the property name in a Filter, and the property name in the Entity map are three unrelated strings
    failure: a rename compiles and returns an empty batch, which is quieter than the DynamoDB ValidationException it corresponds to
  key_is_not_a_property:
    statement: a Datastore key lives beside the properties, not among them
    detail: decoding must fill the key fields from Entity.Key, and encoding must lift them out of the property map
    failure: hand-written code that forgets the lift stores the identity twice and the two copies drift
goals:
  - one declaration produces the codec and the key builder, so kind, key and property names cannot drift
  - no application-field reflection, per decision:reflection-free
  - a type without generated code fails to compile, not at run time
  - driver errors, retry behaviour, transaction restarts and page boundaries stay visible, per rule:firestorebind-driver-passthrough
decisions:
  package_name: firestorebind
  tag_spelling: firestore, per rule:firestore-tag-options
  naming_split: the driver package is datastore and the binding is firestorebind; the product name is the more distinctive of the two, and "datastore" alone says nothing about which datastore
  schema_artifact: none by default, per decision:firestore-no-schema-artifact
  write_result: Store, Insert and their batch forms return the stored key, because an incomplete key comes back completed; Update and Remove stay error-only. This is one place the DynamoDB shape does not carry over, since a PutItem has no key to give back
  transactions: bound, unlike dynamobind, per decision:firestore-transaction-scope
  optimistic_locking: a version tag maps onto the driver's own WithBaseVersion precondition rather than onto a generated condition expression
in_scope:
  - runtime package per decision:firestorebind-runtime-package
  - generator mode emitting per-type entity codecs and key builders, per requirement:firestorebind-generated-entity-codec
  - typed declared queries, per requirement:firestore-typed-queries
  - a typed transaction wrapper, per decision:firestore-transaction-scope
out_of_scope:
  - the client itself; it stays in system:tinygodriver-firestore
  - what the driver excludes: GQL, ReserveIds, SUM and AVG, the admin API, watch, property transformations
  - Firestore native mode, which is a different API with a different client; this binds Datastore mode only
  - composite index management, per decision:firestore-no-schema-artifact
  - an ORM
acceptance:
  - a tagged struct round trips through the driver without the caller naming a property string or building a datastore.Value
  - an entity whose key fields are tagged encodes with the key lifted out of the properties, and decodes with them filled back in
  - an int64 beyond float64 precision survives a round trip, since Integer is text end to end
  - a float64 field and an int64 field produce Double and Integer respectively, and neither is coerced to the other
  - a declared query returns typed values without the caller naming a kind
  - regenerating is unnecessary for a runtime fix, per decision:generated-runtime-in-module
target_state:
  property: no application source names a property, a kind, or a client; every name lives in a tag, a declaration, a Context set once, or generated code
  test: grepping the application for a property name returns nothing, and for a kind name nothing at all, since a kind is intrinsic to the type in a way a table name is not
  stages:
    1_entity_codec: done; requirement:firestorebind-generated-entity-codec, with the codec, the key builder, the kind and the version accessor
    2_declared_queries: not started; requirement:firestore-typed-queries, one named function per access pattern. Until it lands a query names properties as strings through the driver's own builder, which is the quiet failure it exists to make loud
    3_transactions: done; decision:firestore-transaction-scope, a typed closure over the driver's own, plus the three levels of conditional write
    4_index_descriptors: not started; decision:firestore-no-schema-artifact, now unblocked by the v1.1.5 Index type and waiting on stage 2 to have a declaration to hang the clause off
  reading: stage 1 closes the item path; until stage 2 lands a query still names properties as strings, which is the quiet failure requirement:firestore-typed-queries exists to make loud
what_is_easier_than_dynamodb:
  no_single_table_question: a kind belongs to a type by construction, so decision:dynamo-single-table-scope has no counterpart to decline
  no_reserved_words: a filter names a property directly, so there is no expression-attribute-name aliasing to generate
  conditions_are_native: insert-if-absent, update-if-present and baseVersion are driver verbs and options, so requirement:dynamo-optimistic-locking's conflict between a generated condition and a caller's own does not arise
  idempotent_writes: no ADD-shaped update exists, so a retried write is replayable
what_is_harder:
  no_schema_call: nothing to emit in place of decision:dynamobind-table-definition; v1.1.5 added an Index descriptor to emit into, and deriving which index a query needs is still declined upstream and open here, per decision:firestore-no-schema-artifact
  two_number_types: Integer and Double are distinct and not interchangeable in a filter, so data:firestore-property-mapping cannot fold them the way DynamoDB N does
  ancestors: identity is a path, so a key builder is not one or two fields, per decision:firestore-key-identity
  namespaces: a tenancy dimension with no DynamoDB analogue, which lands on the Context rather than on a tag
related:
  - vision:tinybind
  - system:tinybind
  - requirement:tinygo-wasm
  - decision:reflection-free
  - api:firestorebind-operations
  - rule:usage-directed-generation
  - rule:firestore-tag-options
  - concept:dynamobind-firestorebind-mapping
  - concept:code-generation
```
