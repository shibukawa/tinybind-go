---
id: flow:code-generation
type: flow
title: Code Generation Pipeline
---
Generator reads same-package handlers and Go types, then emits local runtime functions and a registered OpenAPI fragment from one IR.

```yaml
flow:
  trigger: developer defines Go types and calls recognized by data:generator-options and data:generator-call-pattern
  steps:
    - id: check-input-hash
      action: hash the run inputs and return the recorded artifact paths unchanged when the generated files already record that hash
      refs:
        - rule:generation-input-hash
        - data:generation-stamp
        - requirement:incremental-generation
    - id: type-check-package
      action: load the analyzed package once for every later phase, after template output is written
      refs:
        - decision:shared-package-load
    - id: discover-handlers
      action: run flow:handler-parse on configured same-package registrations
      refs:
        - concept:handler-discovery
        - concept:route-discovery
        - decision:stdlib-servemux
        - requirement:configurable-generator-discovery
    - id: unwrap-wrappers
      action: unwrap stdlib wrappers and custom middleware when static
      refs:
        - concept:stdlib-wrapper-unwrap
        - rule:nested-wrapper-unwrap
        - rule:custom-middleware-unwrap
    - id: discover-models
      action: map configured direct or framework wrapper calls to semantic operations and retain operation per model
      refs:
        - rule:request-model-discovery
        - rule:response-model-discovery
        - rule:error-response-discovery
        - requirement:configurable-generator-discovery
        - requirement:framework-wrapper-discovery
        - data:generator-call-pattern
    - id: parse-go-types
      action: analyze discovered struct fields and tags
    - id: build-ir
      action: build shared intermediate representation including route metadata
    - id: emit-binders
      action: generate bind* functions only for Bind model usage
      refs:
        - concept:request-binding
        - api:bind
    - id: emit-writers
      action: generate write* and encode helpers only for Write model usage
      refs:
        - concept:response-binding
        - concept:streaming
        - api:write
    - id: emit-standalone-json
      action: generate only DecodeJSON or EncodeJSON paths observed per model
      refs:
        - rule:usage-directed-generation
        - concept:standalone-json-codec
    - id: emit-sql-scanners
      action: generate grouped row scanners for ScanRows model usage
      refs:
        - api:scan-rows
        - rule:sql-group-key
    - id: emit-validation
      action: generate validation logic
    - id: emit-streaming-metadata
      action: generate streaming transport metadata
      refs:
        - concept:streaming
    - id: emit-openapi
      action: generate and register data:openapi-fragment; public handlers serve the merged application document
      refs:
        - concept:openapi-generation
        - concept:openapi-embed
        - requirement:openapi-fragment-aggregation
        - api:openapi-json
        - decision:openapi-31
    - id: stamp-outputs
      action: record the input hash, the output set, and each file's own hash in every written file
      refs:
        - data:generation-stamp
        - rule:generation-input-hash
  invariant: all artifacts derive from the same IR, built from one package type check per run
  related:
    - decision:single-source-of-truth
    - decision:shared-package-load
    - requirement:incremental-generation
    - system:tinybind
    - concept:code-generation
    - flow:handler-parse
    - requirement:openapi-goals
    - rule:usage-directed-generation
```
