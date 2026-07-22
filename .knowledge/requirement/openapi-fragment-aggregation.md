---
id: requirement:openapi-fragment-aggregation
type: requirement
title: OpenAPI Fragment Aggregation
---
Each generated package registers data:openapi-fragment; httpbind public APIs assemble one application OpenAPI document.

```yaml
priority: must
producers:
  - framework packages with framework-specific routes such as health endpoints
  - modular-monolith application packages with package-owned routes
generation:
  - each invocation analyzes only its target Go package
  - generated Go embeds and registers one package fragment
aggregation:
  - api:openapi-assembly and public handlers merge every registered fragment
  - output targets decision:openapi-31
  - JSON and YAML represent the same merged document
application_info:
  - application may set final title and version through public httpbind API
  - generated package fragments do not compete for final info ownership
model: concept:openapi-generation
embedding: concept:openapi-embed
handlers:
  - api:openapi-json
  - api:openapi-yaml
assembly: api:openapi-assembly
rules:
  - sort fragment identities, paths, methods, and component keys deterministically
  - derive component identity from Go package path and type identity
  - identical repeated definitions may deduplicate
  - incompatible path-method or component collisions return an error
  - registration never replaces previously registered fragments
constraints:
  - final application generation does not rescan framework or module dependency source
  - handwritten OpenAPI remains excluded by decision:single-source-of-truth
  - package import makes its generated fragment available before output
acceptance:
  - framework health routes and application routes coexist in final paths
  - independently generated module schemas coexist in final components
  - same-named Go types from different packages receive distinct component identities
  - output is stable across registration order
  - merge conflicts are reported instead of silently overwriting data
```
