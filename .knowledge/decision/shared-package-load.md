---
id: decision:shared-package-load
type: decision
title: One Package Load Per Generation Run
---
Every analysis phase of one run reads a single type-checked package instead of loading the directory itself.

```yaml
status: accepted
context:
  - mapping, configbind, route discovery, and OpenAPI each loaded the same directory
  - type checking dominates generation cost, so the run paid it four times
decision:
  - one lazily created type-checked package per run, shared by every phase
  - load mode is the union of what the phases need
  - package selection excludes test packages
ordering: taken after template generation writes its file, because generated templates join the analyzed package
laziness:
  - a phase disabled by data:generator-options triggers no load
  - the first failing phase reports the load error with its own context, unchanged
consequences:
  - phases observe one consistent view of the package
  - route discovery accepts an already loaded package from the parser boundary
  - measured examples/demo full run 2.45s to 0.79s
invariant: one generation run type-checks the analyzed package at most once
related:
  - requirement:incremental-generation
  - api:generator-execution
  - api:generator-artifacts
  - flow:code-generation
  - flow:handler-parse
  - requirement:modular-package-generation
```
