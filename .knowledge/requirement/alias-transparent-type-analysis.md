---
id: requirement:alias-transparent-type-analysis
type: requirement
title: Alias-Transparent Type Analysis
---
Generator analysis resolves Go type aliases before every named-type test, so an alias binds exactly like the type it names.

```yaml
priority: must
problem: >
  gotypesalias is the go/types default from Go 1.24 and mandatory from Go 1.27,
  so an alias reaches analysis as types.Alias and misses every *types.Named
  assertion
scope:
  packages: [configbind, httpbind, jsonbind, sqlbind]
  sites:
    - config struct field type classification
    - time.Duration detection in concept:config-struct-mapping
    - struct slice element type
    - Bind type argument and wrapper role types
    - method receiver identity under rule:go-types-symbol-identity
rule: unalias before asserting the named type, at every such site, not one
failure_modes_today:
  loud: an alias of a named struct field type reports unsupported field type
  silent: >
    an alias of time.Duration skips duration detection, falls through to
    underlying int64, binds as an integer, and then rejects "5s"
non_goals:
  - resolving a defined type such as 'type D time.Duration' to its source type; only aliases are transparent
  - relaxing the package boundaries of rule:same-package-convention
acceptance:
  - 'type PublicConfig = other.PublicAssetConfig used as a struct field generates the nested table'
  - 'type Timeout = time.Duration binds as a duration and accepts "5s" per rule:duration-value-parsing'
  - 'configbind.Bind[AliasOfConfig]("p") resolves the aliased config type'
  - an alias and the original spelling of one type generate byte-identical code
  - a defined type over a struct keeps its current behavior
related:
  - decision:configbind-supported-types
  - requirement:duration-config-fields
  - requirement:framework-wrapper-discovery
  - rule:duration-value-parsing
  - rule:go-types-symbol-identity
  - rule:same-package-convention
  - concept:config-struct-mapping
  - flow:configbind-codegen
  - flow:code-generation
  - system:configbind
```
