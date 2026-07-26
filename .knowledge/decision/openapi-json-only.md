---
id: decision:openapi-json-only
type: decision
title: OpenAPI Serialized as JSON Only
---
Generated and assembled OpenAPI documents are serialized as JSON only; the library ships no YAML writer.

```yaml
status: accepted
rationale:
  - JSON is the OpenAPI 3.1 native serialization and every consumer reads it
  - one serializer removes a hand-written YAML emitter from the runtime
  - fragments are embedded and merged as JSON already
removed:
  - OpenAPIYAML handler
  - Document YAML method
  - yamlDoc return of api:openapi-assembly
consumers_needing_yaml: convert the served JSON outside the library
related:
  - decision:openapi-31
  - api:openapi-json
  - api:openapi-assembly
  - concept:openapi-generation
```
