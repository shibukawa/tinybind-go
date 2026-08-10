---
id: api:openapi-assembly
type: api
title: OpenAPI Assembly API
---
Public httpbind functions set application info and assemble registered data:openapi-fragment values.

```yaml
signatures:
  - func SetOpenAPIInfo(info OpenAPIInfo) error
  - func AssembleOpenAPI() (jsonDoc []byte, err error)
defaults:
  title: Application API
  version: 0.0.0
errors:
  - missing fragments
  - malformed fragment JSON
  - conflicting fragment identity
  - conflicting path and method
  - conflicting component identity
handlers:
  - api:openapi-json
serialization: decision:openapi-json-only
requirement: requirement:openapi-fragment-aggregation
cached_read_is_unexported:
  fact: api:openapi-json serves from cachedOpenAPI, which assembles once and reuses the result; AssembleOpenAPI is the only public entry and reassembles on every call
  reported: downstream framework survey 2026-08-10, serving the document on the second transport by calling AssembleOpenAPI per request, because a fragment registration cannot be observed from outside the module
  ask: one transport-free cached read, spelled OpenAPIDocument() ([]byte, error), leaving api:openapi-json a thin caller of it
  serves: any framework on any transport, since the document derives from the field plan rather than from the request
  priority: low by the reporter's own account; a documentation endpoint absorbs the reassembly and nothing is blocked
  measured_context: requirement:transport-port-surface records that this surface is the entire remaining difference between the two runtime packages
  shipped_2026_08_10:
    added: OpenAPIDocument() ([]byte, error), the cached read with no transport in its signature
    api_json_now: a thin caller of it, so there is one cache rather than two paths to it
    shared_slice: the returned bytes are the cached document itself and must not be modified; a copy per call would spend the allocation the cache exists to avoid
    verified: two calls return the same backing array, a later fragment registration invalidates it, and an empty registry reports the error rather than an empty document
```
