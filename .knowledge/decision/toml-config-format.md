---
id: decision:toml-config-format
type: decision
title: TOML Config File Format
---
v1 config files use a restricted TOML subset; each Bind prefix maps to a top-level TOML table.

```yaml
status: accepted
format: TOML
table_model: decision:prefix-table-binding
shape: decision:toml-shape-constraints
rationale:
  - human-readable hierarchical config
  - common for Go ops tooling
  - maps cleanly to prefixed nested structs
example:
  - 'Bind[WebServiceConfig]("webservice") reads [webservice]'
  - nested struct fields use [webservice.child] tables
  - primitive arrays use TOML arrays of scalars
out_of_scope_v1:
  - YAML
  - JSON config files
  - HCL
  - inline tables
  - arrays of tables
  - quoted keys
related:
  - requirement:layered-config-load
  - concept:layered-config-sources
  - decision:prefix-table-binding
  - decision:toml-shape-constraints
  - rule:toml-shape-validation
  - system:configbind
  - vision:configbind
```
