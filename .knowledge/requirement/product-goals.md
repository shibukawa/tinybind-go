---
id: requirement:product-goals
type: requirement
title: Product Goals
---
Core product goals for tinybind API design and runtime behavior.

```yaml
goals:
  - Go-first API development
  - reflection-free runtime
  - code generation only
  - unified JSON / form / multipart handling
  - standalone typed JSON read/write (not HTTP-only)
  - non-silent diagnostics for undiscoverable routes
  - explicit success HTTP status via WriteStatus
  - browser-friendly APIs
  - curl-friendly APIs
  - RFC 9457 compliant errors
  - automatic OpenAPI generation
  - TinyGo compatible
  - type-safe streaming APIs
maps_to:
  - decision:reflection-free
  - decision:single-source-of-truth
  - decision:stdlib-servemux
  - concept:request-binding
  - concept:standalone-json-codec
  - concept:streaming
  - concept:net-http-handler
  - policy:problem-details
  - concept:openapi-generation
  - requirement:tinygo-wasm
  - api:bind
  - api:write
  - api:write-error
  - api:decode-json
  - api:encode-json
  - api:write-status
  - requirement:analysis-diagnostics
  - requirement:bind-check-validation
  - rule:analysis-diagnostics-check
  - rule:same-package-convention
  - rule:openapi-success-status
```

