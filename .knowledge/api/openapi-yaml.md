---
id: api:openapi-yaml
type: api
title: OpenAPI YAML Handler
---
Public handler serializes the document merged from every registered data:openapi-fragment as YAML.

```yaml
signature: |
  func OpenAPIYAML(
      w http.ResponseWriter,
      r *http.Request,
  )
recommended_mount: "GET /openapi.yaml"
example: |
  mux.HandleFunc(
      "GET /openapi.yaml",
      httpbind.OpenAPIYAML,
  )
source: concept:openapi-embed
aggregation: requirement:openapi-fragment-aggregation
assembly: api:openapi-assembly
related:
  - api:openapi-json
  - concept:openapi-generation
  - concept:openapi-ui
```
