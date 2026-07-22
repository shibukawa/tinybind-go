---
id: api:openapi-assembly
type: api
title: OpenAPI Assembly API
---
Public httpbind functions set application info and assemble registered data:openapi-fragment values.

```yaml
signatures:
  - func SetOpenAPIInfo(info OpenAPIInfo) error
  - func AssembleOpenAPI() (jsonDoc []byte, yamlDoc []byte, err error)
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
  - api:openapi-yaml
requirement: requirement:openapi-fragment-aggregation
```
