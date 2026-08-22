---
id: decision:runtime-package-boundaries
type: decision
title: Runtime Package Boundaries
---
Separate runtime APIs by platform dependency so importing one mapping mode does not compile unrelated standard-library paths.

```yaml
packages:
  jsonbind:
    path: github.com/shibukawa/tinybind-go/jsonbind
    owns:
      - api:decode-json
      - api:encode-json
      - JSON codec registry and document helpers
    excludes:
      - net/http
      - database/sql
  httpbind:
    path: github.com/shibukawa/tinybind-go
    owns:
      - api:bind
      - api:write
      - HTTP registry, errors, streaming, and OpenAPI serving
    may_import:
      - jsonbind
  fasthttpbind:
    status: proposed by decision:fasthttpbind-runtime-package
    path: github.com/shibukawa/tinybind-go/fasthttpbind
    owns:
      - api:fasthttpbind-bind
      - api:fasthttpbind-write
      - api:fasthttpbind-stream
    imports:
      - github.com/shibukawa/tinygodriver/fasthttp
      - jsonbind
    excludes:
      - net/http
      - database/sql
    note: the second transport runtime; it never shares a package with httpbind, so no binary links both
  sqlbind:
    path: github.com/shibukawa/tinybind-go/sqlbind
    owns:
      - api:scan-rows
      - SQL scanner registry and row helpers
      - data:sql-statement, Execer, Querier, Builder, and AppendValues used by generated SQL
      - decision:sql-context-executor-api standard resolver
    excludes:
      - net/http
  htmlbind:
    owns:
      - decision:generated-render-plan coordinator
      - requirement:head-merging and requirement:chain-render-pipeline execution
      - trusted value types, Escape, and the scalar and JSON formatters used by generated HTML
    excludes:
      - net/http
    note: requirement:html-component-api is HTTP-independent, so the HTML runtime stays a transport-neutral leaf
  cborbind:
    status: implemented 2026-08-22 by requirement:cbor-codec-generation; a first version existed 2026-08-19 to 2026-08-20
    path: github.com/shibukawa/tinybind-go/cborbind
    owns:
      - the four entry points of requirement:cbor-codec-generation, their shape-named interfaces, and ErrShape
      - no spelling of the driver's codec interfaces, per rule:cbor-codec-interface-upstream
    imports: nothing at all
    imports_nothing_was_not_the_plan:
      expected: the driver, as the dynamobind and firestorebind shape
      as_built: the entry points and interfaces are spelled in byte slices, so the driver is named only by the generated code that calls it
      consequence: a package importing cborbind alone links no CBOR implementation, which makes this the lightest runtime here rather than the third driver-dependent one
    excludes:
      - net/http
      - database/sql
  dynamobind:
    status: proposed by decision:dynamobind-runtime-package
    path: github.com/shibukawa/tinybind-go/dynamobind
    owns:
      - api:dynamobind-operations
      - ItemEncoder, ItemDecoder, and Keyer used by generated DynamoDB codecs
    imports:
      - github.com/shibukawa/tinygodriver/nosql/dynamodb
    excludes:
      - database/sql
  firestorebind:
    status: proposed by decision:firestorebind-runtime-package
    path: github.com/shibukawa/tinybind-go/firestorebind
    owns:
      - api:firestorebind-operations
      - EntityEncoder, EntityDecoder, and Keyer used by generated Firestore codecs
      - the Context client and namespace resolution of decision:firestore-context-client-api
      - the typed transaction wrapper of decision:firestore-transaction-scope
    imports:
      - github.com/shibukawa/tinygodriver/nosql/datastore
    excludes:
      - database/sql
    name_note: the only runtime named after its service rather than its driver package, per decision:firestorebind-runtime-package
dependency_direction:
  - httpbind -> jsonbind
  - sqlbind remains independent unless it needs a transport-neutral leaf
  - dynamobind -> system:tinygodriver-dynamodb, and firestorebind -> system:tinygodriver-firestore; the two runtimes that depend on an external driver, and the two that share no code with each other
  - generated CBOR HTTP codecs -> system:tinygodriver-cbor, with no runtime package of their own; requirement:cbor-http-body is a generator option rather than an imported runtime
  - user package -> cborbind, which depends on nothing, and user package -> system:tinygodriver-cbor directly, because the generated codecs name driver types; the edge to the driver is the generated file's, not the runtime's
forbidden:
  - jsonbind -> httpbind
  - tinygodriver -> tinybind-go, in any package, example, or test
  - shared runtime code importing net/http or database/sql for every mode
  - generated code declaring its own copy of a runtime type or helper, per decision:generated-runtime-in-module
generation:
  JSON-only: import and register with jsonbind
  HTTP: import httpbind and jsonbind; register each entry with its owner
  SQL-only: import and register with sqlbind
  DynamoDB-only: import the driver for the emitted methods; no registration, per decision:dynamobind-static-dispatch
  Firestore-only: import the driver for the emitted methods; no registration, for the same reason
generator:
  command: cmd/tinybind-gen
  mapping_file: tinybind_gen.go
  openapi_file: tinybind_openapi_gen.go
compatibility: root JSON primitive helpers delegate to jsonbind; generic DecodeJSON / EncodeJSON live only in jsonbind
reason: requirement:tinygo-wasm
related:
  - concept:standalone-json-codec
  - rule:usage-directed-generation
  - decision:single-source-of-truth
```
