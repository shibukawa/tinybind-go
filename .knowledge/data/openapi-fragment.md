---
id: data:openapi-fragment
type: data
title: OpenAPI Package Fragment
---
Generated OpenAPI operations and schemas contributed by one Go package before final document assembly.

```yaml
identity:
  - Go package path
contains:
  - path and HTTP method operations
  - referenced component schemas
  - validation, response, error, and streaming metadata
excludes:
  - final JSON or YAML document
  - application-level OpenAPI info ownership
merge_conflicts:
  - duplicate path and method with different operations
  - duplicate component identity with different schemas
component_identity: package-qualified Go type identity with deterministic document naming
```
