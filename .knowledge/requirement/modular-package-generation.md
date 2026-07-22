---
id: requirement:modular-package-generation
type: requirement
title: Modular Package Generation
---
Framework and application packages are generated independently; imported generated artifacts compose without rescanning dependency source.

```yaml
priority: must
scenario:
  - framework package generates framework routes and config once
  - modular-monolith packages run generation for their own handlers and config
  - final application imports framework and module packages
rules:
  - one generator invocation analyzes one Go package
  - generated dependency packages remain reusable without regeneration by the application
  - package-local runtime functions remain package-local
  - whole-application artifacts use registered package fragments plus a public aggregation API
  - generated registry identities include Go package path and type identity, not bare type name alone
aggregate_artifacts:
  - requirement:scaffold-generation
  - requirement:openapi-fragment-aggregation
package_local_artifacts:
  - request binders
  - response writers and JSON codecs
  - validation and SQL scanners
  - HTML and SQL template wrappers
constraints:
  - no final invocation scans all dependency source trees
  - no last-registration-wins behavior for aggregate artifacts
  - aggregate output is deterministic and independent of package init order
acceptance:
  - framework-only generated fragments remain present after application package generation
  - N independently generated module packages contribute to one final config scaffold and OpenAPI document
  - same-named types in different packages coexist without registry replacement
  - package-local artifacts require no central aggregation step
```
