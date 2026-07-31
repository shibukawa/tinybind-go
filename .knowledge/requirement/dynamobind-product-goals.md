---
id: requirement:dynamobind-product-goals
type: requirement
title: dynamobind Product Goals
---
Generate DynamoDB item codecs, key builders, and table schema from one Go struct declaration so no call site handles a raw attribute map.

```yaml
status: required
implemented: 2026-07-31; runtime in dynamobind/, generator mode in generator/dynamobind*.go, fixture in internal/dynamofixture
source: user design request 2026-07-31; tinygodriver DynamoDB exploration
target: system:tinygodriver-dynamodb
problems:
  drift:
    statement: nothing connects a struct to its table
    detail: TableDefinition.PartitionKey.Name, the item tag, and the Key passed to GetItem are three unrelated strings
    failure: a rename compiles and fails at run time with ValidationException
  reflection_cost:
    statement: the driver reflection path costs binary size and time for a struct known at compile time
    measured: 24 KB linked, 0.8 us and 21 allocations per MarshalItem
goals:
  - one declaration produces codec, key builder, and table definition, so they cannot drift
  - no application-field reflection, per decision:reflection-free
  - a type without generated code fails to compile, not at run time
  - driver errors, retry behavior, and pagination stay visible, per rule:dynamobind-driver-passthrough
decisions:
  package_name: dynamobind
  tag_spelling: dynamo, per rule:dynamo-tag-options
  table_definition: emitted, per decision:dynamobind-table-definition
  write_result: Store and Remove stay error-only; StoreReturning and RemoveReturning decode the old item
  batch_chunking: the runtime, at the fixed limits
  iterators: Query and Scan ship, with the request cost documented and QueryPage public
  partial_update: a thin Update wrapper supplying only the key; no generated expressions
in_scope:
  - runtime package per decision:dynamobind-runtime-package
  - generator mode emitting per-type codecs and key builders, per requirement:dynamobind-generated-item-codec
  - table definition emission, per decision:dynamobind-table-definition
out_of_scope:
  - the DynamoDB client itself; it stays in system:tinygodriver-dynamodb
  - removing encoding/json from the request path, per decision:dynamobind-json-transport-deferred
  - what the driver excludes: transactions, PartiQL, Streams, DAX, per system:tinygodriver-dynamodb transaction_note
  - single-table design, per decision:dynamo-single-table-scope
  - secondary index key tags; defer until the primary key path is proven
acceptance:
  - a tagged struct round trips through the driver without the caller naming an attribute string
  - generated path stays within the size budget in requirement:dynamobind-verification
  - regenerating is unnecessary for a runtime fix, per decision:generated-runtime-in-module
target_state:
  property: no application source names a DynamoDB attribute, and a declared query names no table either; every name lives in a tag, a declaration or generated code
  test: grepping the application for an attribute name returns nothing, and for a table name only where an item operation is called without a declaration
  reached:
    - the item path, where the codec, the key builder and the table definition all come from the tags
    - the read path of a declared query, whose attributes, placeholders and reserved-word aliases are all generated
    - the table name of a declared query, which comes from its table clause
    - the client of a declared query, when decision:dynamo-context-client-api is on
  not_reached:
    - the table name of an item operation, which has no declaration to read one from
    - a key condition written as text, which stays unchecked as the escape hatch
  stages:
    1_item_codec: done
    2_declared_queries: done; requirement:dynamo-typed-queries generates one named function per access pattern, closing the read path for a primary key and with it the reserved-word hazard
    3_index_tags: names the index a query runs against; deferred, and defined for the multi-table model only per decision:dynamo-single-table-scope
    4_open_expressions: filter, condition and update expressions, the grammar that has to be authored; a filter joins the stage 2 declaration rather than needing its own mechanism
  reading: stage 2 is where the property holds for anyone reading and writing by primary key, which is most callers; the later stages raise the ceiling rather than the floor
not_the_goal:
  - an ORM
  - single-table design, per decision:dynamo-single-table-scope
  - transactions, which the driver does not expose and which would not be a single-type generic if it did
  - hiding the driver; errors, retries and page boundaries stay the caller's, per rule:dynamobind-driver-passthrough
planned:
  - requirement:dynamo-optimistic-locking
  - requirement:dynamo-ttl-attribute, blocked on the driver
  scope: decision:dynamo-framework-requests
deferred:
  - a page-level Query and Scan iterator, per api:dynamobind-operations
  - generated update and condition expressions, which requirement:dynamo-typed-queries does not reopen
  - secondary index tags, per rule:dynamo-tag-options
related:
  - vision:tinybind
  - system:tinybind
  - requirement:tinygo-wasm
  - decision:reflection-free
  - api:dynamobind-operations
  - rule:usage-directed-generation
```
