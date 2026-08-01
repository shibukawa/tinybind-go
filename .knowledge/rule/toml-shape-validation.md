---
id: rule:toml-shape-validation
type: rule
title: TOML Shape Validation
---
Reject TOML constructs outside the accepted shape before mapping to structs.

```yaml
must_accept:
  - bare key = value pairs
  - dotted bare keys as nested path sugar
  - nested standard tables under Bind prefix
  - arrays of primitive scalars only
  - arrays of tables [[key]] as slices of structs
must_reject:
  - quoted keys
  - inline tables
  - a [table] header that reopens an array of tables
  - a key whose ancestor is an array of tables reached from outside its element
  - a scalar where the target field is a slice of structs
on_reject: load error with diagnostics identifying the forbidden construct
applies_to:
  - flow:config-load
  - decision:toml-shape-constraints
related:
  - decision:toml-config-format
  - decision:configbind-supported-types
  - concept:config-struct-mapping
```
