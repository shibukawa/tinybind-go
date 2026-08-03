---
id: concept:dynamobind-firestorebind-mapping
type: concept
title: What Transfers From dynamobind To firestorebind, And What Does Not
---
"Same as DynamoDB" is a statement about shape and effort, not about parity. This records which dynamobind concepts have a Firestore counterpart, which invert, and which have none, so the gaps are decided once rather than rediscovered per operation.

```yaml
upstream_counterpart: the driver's own DynamoDB-to-Datastore comparison concept, which covers the client layer; this one is about the binding layer above it
transfers_unchanged:
  generated_codec: one declaration produces the codec, and a stale file is a build failure; requirement:firestorebind-generated-entity-codec mirrors requirement:dynamobind-generated-item-codec
  static_dispatch: pointer-constraint generics, no registry, no init entry; decision:dynamobind-static-dispatch applies as written
  usage_direction: rule:usage-directed-generation, unchanged
  runtime_in_module: decision:generated-runtime-in-module, unchanged
  driver_passthrough: rule:firestorebind-driver-passthrough restates rule:dynamobind-driver-passthrough with more to pass through
  context_client: decision:firestore-context-client-api restates decision:dynamo-context-client-api, minus the name resolver
  declared_queries: requirement:firestore-typed-queries restates requirement:dynamo-typed-queries, minus the table clause
  tag_discipline: an unknown option is a generation error; rule:firestore-tag-options restates rule:dynamo-tag-options
transfers_after_v1_1_5:
  reflection_fallback:
    was: the v1.1.4 driver shipped no struct mapper, so this concept recorded the absence as an inversion and requirement:firestorebind-product-goals argued from it
    now: v1.1.5 added MarshalEntity and UnmarshalEntity, so both drivers have a reflection path and the situation transfers rather than inverting
    including_the_hazard: the two-tag problem transfers too, and it was reported from the DynamoDB side before it existed here; rule:firestore-tag-options carries the same check rule:dynamo-tag-options does, at the driver's own suggestion
    what_still_differs: DynamoDB's dynamodbav honours only "-" and omitempty and silently reads the rest as nothing, while the Datastore mapper honours noindex and omitempty and rejects maps outright, so the driver path is less lossy here
inverts:
  transactions:
    dynamobind: excluded, because the driver declares none
    firestorebind: included, because they are the only conditional path; decision:firestore-transaction-scope
  conditional_writes:
    dynamodb: a condition expression the generator has to build, and requirement:dynamo-optimistic-locking has to reserve the one expression slot against the caller's own
    firestore: verbs and a baseVersion field, so Insert, Update and a version tag compose with a caller's options instead of competing with them
  identity_in_the_payload:
    dynamodb: the partition key is an item attribute, so the codec writes it like any other
    firestore: the key sits beside the properties, so the codec lifts it out on encode and fills it back on decode; decision:firestore-key-identity
  where_the_name_comes_from:
    dynamodb: a table is a deployment fact, named per statement and remapped at run time
    firestore: a kind is intrinsic to the type, so nothing names it and nothing remaps it
no_counterpart_in_firestorebind:
  table_definition: decision:dynamobind-table-definition has nothing to emit; decision:firestore-no-schema-artifact
  single_table_scope: decision:dynamo-single-table-scope declines a design that cannot arise when a kind belongs to a type
  ttl_tag: requirement:dynamo-ttl-attribute; Datastore-mode expiry is a policy over an ordinary timestamp property, applied out of band, so no tag produces it
  set_encodings: stringset, numberset and binaryset name a type Datastore does not have
  unixtime: Datastore has a real timestamp type
  returning_forms: no commit returns a prior entity, so StoreReturning and RemoveReturning of api:dynamobind-operations have nothing to decode
  reserved_word_aliasing: a filter names a property directly, so there are no expression attribute names to generate
  scan: a kind-only query is what Scan was, so there is no second entry point
no_counterpart_in_dynamobind:
  ancestors: identity is a path, and an ancestor query is the only strongly consistent multi-entity read
  namespaces: a tenancy dimension that lands on the Context
  unindexed_properties: every property is indexed by default, so noindex is a real choice with a throughput cost
  two_number_types: Integer and Double are distinct and not interchangeable in a filter or an order
  composite_indexes: a typed query can still fail for want of an index; v1.1.5 added a descriptor to declare one and declined to derive which query needs one, so the declaration is the author's, per decision:firestore-no-schema-artifact
  chunking_by_size: DynamoDB publishes batch counts and Datastore publishes only byte bounds, so api:firestorebind-operations chunks a batch write by encoded size where api:dynamobind-operations chunks by count
  geo_points: no DynamoDB analogue
  deferred_lookup_keys: distinct from missing keys, and the pair has no single DynamoDB equivalent
what_the_two_runtimes_share_in_code:
  answer: nothing
  why: Item and Entity are unrelated driver types with unrelated codecs, so a shared generic would have to erase the key model difference decision:firestore-key-identity exists to keep visible
  what_is_actually_shared: the generator's discovery, usage direction and diagnostic machinery, which is not runtime code
consequence: >
  a reader who knows dynamobind can predict the file layout, the tag discipline
  and the call-site property, and should predict none of the write path. The
  three inversions above are where a ported assumption goes wrong.
related:
  - requirement:firestorebind-product-goals
  - requirement:dynamobind-product-goals
  - system:tinygodriver-firestore
  - system:tinygodriver-dynamodb
  - decision:firestorebind-runtime-package
```
