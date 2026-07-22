---
id: concept:openapi-embed
type: concept
title: Embedded OpenAPI Fragments
---
Generator embeds one data:openapi-fragment per package; httpbind assembles registered fragments for runtime serving.

```yaml
generated_file_example: tinybind_openapi_gen.go
embedded_symbol_example: "var generatedOpenAPIFragment = ..."
includes:
  - package-local operations
  - package-local schema metadata
aggregation: requirement:openapi-fragment-aggregation
handlers:
  - api:openapi-json
  - api:openapi-yaml
related:
  - concept:openapi-generation
  - decision:openapi-31
  - requirement:openapi-goals
```
