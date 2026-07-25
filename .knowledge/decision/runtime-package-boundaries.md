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
  sqlbind:
    path: github.com/shibukawa/tinybind-go/sqlbind
    owns:
      - api:scan-rows
      - SQL scanner registry and row helpers
    excludes:
      - net/http
  htmlbind:
    owns:
      - decision:generated-render-plan coordinator
      - requirement:head-merging and requirement:chain-render-pipeline execution
    excludes:
      - net/http
    note: requirement:html-component-api is HTTP-independent, so the HTML runtime stays a transport-neutral leaf
dependency_direction:
  - httpbind -> jsonbind
  - sqlbind remains independent unless it needs a transport-neutral leaf
forbidden:
  - jsonbind -> httpbind
  - shared runtime code importing net/http or database/sql for every mode
generation:
  JSON-only: import and register with jsonbind
  HTTP: import httpbind and jsonbind; register each entry with its owner
  SQL-only: import and register with sqlbind
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
