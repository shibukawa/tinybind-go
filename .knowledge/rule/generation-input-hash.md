---
id: rule:generation-input-hash
type: rule
title: Generation Input Hash
---
Hash every input a run depends on before doing work, and skip the run when the generated files already record that hash.

```yaml
hashed_inputs:
  - non-test Go sources in the analyzed directory
  - files matching the configured template patterns
  - go.mod and go.sum of the containing module
  - directory path inside the module, which fixes the generated registration identity
  - normalized data:generator-options, artifact names, and per-run switches
  - content of the running generator executable
excluded_inputs:
  - files this run writes, listed as data:generation-stamp outputs
  - _test.go files, which no generation phase loads
generator_identity:
  value: hash of the executable content
  reason: go run reports no usable version yet links a binary that follows generator sources
skip_requires_all:
  - a candidate output records the current inputs value
  - every name in that stamp's outputs exists
  - each of those files matches its own recorded self hash
regenerate_when_any:
  - a hashed input changed, appeared, or disappeared
  - an output is missing, unstamped, edited, or truncated
  - the stamp version is unrecognized
  - the run requests force
skipped_run:
  - returns the recorded artifact paths
  - reports the skip
  - performs no package load
scope: analyzed directory only, per requirement:modular-package-generation
undetected: a change in another package that still reaches this package's generated output; force regenerates
never: reuse an output whose recorded inputs or self hash cannot be verified
related:
  - data:generation-stamp
  - requirement:incremental-generation
  - api:generator-execution
  - api:generator-main
  - flow:code-generation
```
