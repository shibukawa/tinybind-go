---
id: requirement:per-source-generation-artifacts
type: requirement
title: Per-Source Generation Artifacts
---
Generated code is addressable per owning source file so a framework CLI can place each artifact beside the source that produced it.

```yaml
priority: must
source: downstream framework CLI requirements for tinybind v0.1.12
problem:
  - template generation merges every HTML and SQL source of a package into one fixed artifact name
  - binding, configbind, and OpenAPI outputs use fixed package-level names
  - a caller cannot express which source owns which generated declaration
  - renaming only the aggregate output keeps source ownership inaccurate
rules:
  - analysis results are retrievable as data:generation-artifact values
  - each artifact carries owning source path, suggested output base, artifact kind, and generated Go source
  - no package-shared artifact exists; runtime support is imported from the module per decision:generated-runtime-in-module
  - the caller derives the final file name from OutputBase, for example '{base}_pw_gen.go'
  - api:generator-artifacts returns the same artifacts without writing files, which is sufficient for check mode
  - aggregation into one package output remains available through api:generator-execution
isolation:
  - generated identifiers are unique across all artifacts of one package
  - per-source artifacts do not redeclare a shared type, helper, or variable
  - a generated artifact declares nothing that is not derived from its own source, so uniqueness needs no hoisting pass or runtime-name list
  - config definition registrar names carry the package-wide spec index
  - init registrations from separate artifacts of one package coexist without conflict
  - emitting a subset of a package's artifacts never breaks the emitted subset's compilation of its own declarations
  - artifact content is stable when unrelated sources of the same package change
acceptance:
  - one HTML source yields exactly one html_template artifact naming that source
  - one SQL source yields exactly one sql_template artifact naming that source
  - one Go source with recognized calls yields binding and configbind artifacts naming that source
  - writing every returned artifact of a package compiles and passes go test
  - regenerating without source change produces byte-identical artifacts
  - two SQL sources in one package generate without a duplicate-declaration failure
  - no artifact named after the removed shared base is produced
related:
  - decision:generated-runtime-in-module
  - api:generator-artifacts
  - data:generation-artifact
  - api:generator-execution
  - requirement:modular-package-generation
  - requirement:configurable-template-file-patterns
  - rule:generated-source-self-contained
  - requirement:custom-framework-generation-profile
```
