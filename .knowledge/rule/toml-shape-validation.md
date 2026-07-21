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
must_reject:
  - quoted keys
  - inline tables
  - arrays of tables
on_reject: load error with diagnostics identifying the forbidden construct
applies_to:
  - flow:config-load
  - decision:toml-shape-constraints
related:
  - decision:toml-config-format
  - decision:configbind-supported-types
  - concept:config-struct-mapping
```
