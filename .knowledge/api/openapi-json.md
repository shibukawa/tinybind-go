---
id: api:openapi-json
type: api
title: OpenAPI JSON Handler
---
Public handler serializes the document merged from every registered data:openapi-fragment as JSON.

```yaml
signature: |
  func OpenAPIJSON(
      w http.ResponseWriter,
      r *http.Request,
  )
recommended_mount: "GET /openapi.json"
example: |
  mux.HandleFunc(
      "GET /openapi.json",
      httpbind.OpenAPIJSON,
  )
source: concept:openapi-embed
aggregation: requirement:openapi-fragment-aggregation
assembly: api:openapi-assembly
related:
  - api:openapi-yaml
  - concept:openapi-generation
  - concept:openapi-ui
```
