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
decided_2026_07_31:
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
  - what the driver excludes: transactions, PartiQL, Streams, DAX
  - secondary index key tags; defer until the primary key path is proven
acceptance:
  - a tagged struct round trips through the driver without the caller naming an attribute string
  - generated path stays within the size budget in requirement:dynamobind-verification
  - regenerating is unnecessary for a runtime fix, per decision:generated-runtime-in-module
open: none; every design choice is resolved and the remaining work is implementation
deferred:
  - a page-level Query and Scan iterator, per api:dynamobind-operations
  - generated update and condition expressions
  - secondary index tags, per rule:dynamo-tag-options
related:
  - vision:tinybind
  - system:tinybind
  - requirement:tinygo-wasm
  - decision:reflection-free
  - api:dynamobind-operations
  - rule:usage-directed-generation
```
