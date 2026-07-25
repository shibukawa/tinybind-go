---
id: rule:generated-source-self-contained
type: rule
title: Self-Contained Generated Source
---
Every Go source returned or written by a generator API is already formatted and imports exactly what it uses.

```yaml
priority: must
applies_to:
  - api:generator-execution
  - api:generator-artifacts
  - api:generator-main generate command
applies_only_to: data:generation-artifact entries with a go_package destination; a requirement:static-asset-extraction public asset is written verbatim
rules:
  - output passes go/format before it is returned or written
  - the import block contains every package the artifact references
  - the import block contains no package the artifact does not reference
  - import correctness holds per artifact, not only for the full package set
  - callers need no goimports-equivalent post-processing
  - callers never rewrite generated source to change its meaning
derivation:
  mapping: imports follow qualifier use in the emitted body, not per-type usage flags
  templates: a template artifact imports the module runtime packages it calls and nothing else, per decision:generated-runtime-in-module
phase_combinations:
  - template only
  - SQL only
  - config only
  - binding only
  - OpenAPI disabled
  - custom wrapper calls only
acceptance:
  - each phase combination compiles and passes go test or go test -run=^$
  - no artifact triggers an unused-import or undefined-identifier compile error
  - enabling an unrelated phase does not change another artifact's import block
related:
  - requirement:per-source-generation-artifacts
  - data:generation-artifact
  - rule:usage-directed-generation
  - rule:generator-feature-disable
```
