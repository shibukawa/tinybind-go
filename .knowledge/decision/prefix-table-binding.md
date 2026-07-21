---
id: decision:prefix-table-binding
type: decision
title: Prefix Table Binding
---
Each bound struct owns a TOML table named by its Bind prefix; fields map under that table.

```yaml
status: accepted
api_shape: 'c := configbind.Bind[WebServiceConfig]("webservice")'
toml_table: "[webservice]"
semantics:
  - Bind[T](prefix) registers type T under prefix
  - TOML keys for T live under table named prefix
  - env and CLI names are derived with the same prefix
  - multiple Bind calls use different prefixes for split config surfaces
example:
  go: |
    c := configbind.Bind[WebServiceConfig]("webservice")
  toml: |
    [webservice]
    listen_addr = ":8080"
    read_timeout = "5s"
    cors_origins = ["https://a.example"]
    [webservice.tls]
    enabled = true
nested_structs: standard nested tables only; no inline tables
related:
  - api:configbind-bind
  - requirement:struct-registration
  - concept:config-struct-mapping
  - decision:toml-config-format
  - decision:toml-shape-constraints
  - system:configbind
```
