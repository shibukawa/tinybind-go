---
id: concept:openapi-ui
type: concept
title: Optional OpenAPI UI
---
Optional embedded documentation UIs are runtime packages and are not required by the OpenAPI generator.

```yaml
recommended_docs_path: "GET /docs/"
optional_uis:
  - Swagger UI
  - Redoc
  - Scalar
example: |
  mux.Handle(
      "GET /docs/",
      httpbind.SwaggerUI("/openapi.json"),
  )
mount_freedom: applications may mount handlers at any path
related:
  - api:openapi-json
  - concept:openapi-embed
  - concept:openapi-generation
```
