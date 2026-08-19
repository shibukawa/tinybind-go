---
id: system:tinybind
type: system
title: tinybind-go Library
---
TinyGo-oriented Go library and generator with dependency-isolated HTTP, JSON, and SQL binding runtimes.

```yaml
runtime_packages:
  - github.com/shibukawa/tinybind-go/jsonbind: standalone JSON codec runtime
  - github.com/shibukawa/tinybind-go: package httpbind; net/http runtime
  - github.com/shibukawa/tinybind-go/fasthttpbind: proposed decision:fasthttpbind-runtime-package fasthttp runtime over system:tinygodriver-fasthttp
  - github.com/shibukawa/tinybind-go/sqlbind: database/sql runtime
  - github.com/shibukawa/tinybind-go/dynamobind: proposed decision:dynamobind-runtime-package DynamoDB item runtime over system:tinygodriver-dynamodb
  - github.com/shibukawa/tinybind-go/cborbind: proposed decision:cborbind-runtime-package CBOR codec runtime over system:tinygodriver-cbor
generator_command: cmd/tinybind-gen
package_boundary: decision:runtime-package-boundaries
runtime_style: generated code only; no reflection
public_api:
  - api:bind
  - api:write
  - api:write-status
  - api:write-error
  - api:decode-json
  - api:encode-json
  - concept:error-helpers
  - concept:standalone-json-codec

primary_inputs:
  - developer-defined Go types
  - struct field tags for source restriction
  - same-package net/http handlers
  - io.Reader / io.Writer for standalone JSON
outputs:
  - generated bind and write functions
  - validation code
  - OpenAPI schemas
  - streaming metadata
  - typed JSON codecs for registered models
related:
  - vision:tinybind
  - flow:code-generation
  - flow:handler-request
  - concept:request-binding
  - concept:response-binding
  - concept:standalone-json-codec
  - concept:streaming
  - concept:net-http-handler
  - concept:handler-discovery
  - flow:handler-parse
  - concept:stdlib-wrapper-unwrap
  - policy:problem-details
  - decision:stdlib-servemux
  - concept:openapi-generation
  - concept:openapi-embed
  - api:openapi-json
  - api:decode-json
  - api:encode-json
  - api:write-status
  - requirement:analysis-diagnostics
  - rule:analysis-diagnostics-check
  - rule:same-package-convention
  - decision:runtime-package-boundaries
  - requirement:dynamobind-product-goals
  - api:dynamobind-operations
  - requirement:declared-cbor-codec
  - decision:cborbind-runtime-package
```
