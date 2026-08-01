---
id: requirement:incremental-generation
type: requirement
title: Incremental Generation
---
Repeated generation costs only what changed; a run whose inputs are unchanged writes nothing.

```yaml
priority: must
scenario:
  - go generate over a repository whose packages are mostly unchanged
  - watch or editor integration reruns generation after one edit
  - CI regenerates to verify that committed artifacts are current
rules:
  - a run records the inputs it read into the files it writes
  - a later run reading identical inputs returns the recorded artifact paths and writes nothing
  - a skipped run performs no package type check
  - one run type-checks the analyzed package at most once
  - skipping is reported and can be overridden per run
cost_model:
  dominant: package type checking
  unchanged_package: input hashing only
  template_compilation: minor relative to type checking
scope: one analyzed directory, per requirement:modular-package-generation
non_goals:
  - dependency tracking across packages
  - a cache stored outside the generated files
  - reusing an output whose recorded inputs are unknown or stale
acceptance:
  - unchanged package returns the same artifact paths without rewriting them
  - edited source, template, module file, option, or generator binary regenerates
  - deleted, edited, or truncated generated file regenerates
  - test file edits do not regenerate
mechanism:
  - rule:generation-input-hash
  - data:generation-stamp
  - decision:shared-package-load
```
