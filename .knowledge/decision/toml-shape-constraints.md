---
id: decision:toml-shape-constraints
type: decision
title: TOML Shape Constraints
---
v1 accepts only a restricted TOML subset aligned with nested structs, primitive arrays, and struct slices.

```yaml
status: accepted
allowed:
  - standard tables [prefix] and nested [prefix.child]
  - bare keys only (unquoted pair keys)
  - dotted bare keys as sugar for nested tables
  - scalar values of supported types
  - arrays of primitive scalars
  - nested structs via nested tables or dotted bare keys
  - arrays of tables [[prefix.child]] for slices of structs
  - standard tables under an open [[array]] element as that element's sub-table
forbidden:
  - quoted keys
  - inline tables
  - arrays of inline tables
  - reopening an array of tables with a plain [table] header
  - keys reaching through an array of tables from outside its element
rationale:
  - maps 1:1 to Go nested structs without dynamic map shapes
  - dotted bare keys are equivalent to nested standard tables
  - repeated settings are data, so a slice of structs needs a repeated table
  - table-array elements stay nested sub-documents instead of index-encoded keys
  - keeps parser and codegen surface small for TinyGo
examples:
  allowed_toml: |
    [webservice]
    listen_addr = ":8080"
    cors_origins = ["https://a.example", "https://b.example"]
    tls.enabled = true
    [webservice.tls]
    cert_path = "server.crt"
  allowed_dotted: 'webservice.tls.enabled = true under prefix scope'
  allowed_array_of_tables: |
    [[webservice.listeners]]
    addr = ":8080"
    tls.enabled = true
    [webservice.listeners.tls]
    cert_path = "server.crt"
  forbidden_inline_table: 'tls = { enabled = true }'
  forbidden_quoted_key: '"listen-addr" = ":8080"'
related:
  - decision:toml-config-format
  - decision:configbind-supported-types
  - decision:prefix-table-binding
  - rule:toml-shape-validation
  - concept:config-struct-mapping
  - system:configbind
```
