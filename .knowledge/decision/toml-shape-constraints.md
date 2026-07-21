---
id: decision:toml-shape-constraints
type: decision
title: TOML Shape Constraints
---
v1 accepts only a restricted TOML subset aligned with nested structs and primitive arrays.

```yaml
status: accepted
allowed:
  - standard tables [prefix] and nested [prefix.child]
  - bare keys only (unquoted pair keys)
  - dotted bare keys as sugar for nested tables
  - scalar values of supported types
  - arrays of primitive scalars
  - nested structs via nested tables or dotted bare keys
forbidden:
  - quoted keys
  - inline tables
  - arrays of tables
  - arrays of inline tables
rationale:
  - maps 1:1 to Go nested structs without dynamic map shapes
  - dotted bare keys are equivalent to nested standard tables
  - avoids ambiguous merge of table arrays
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
  forbidden_inline_table: 'tls = { enabled = true }'
  forbidden_array_of_tables: |
    [[webservice.listeners]]
    addr = ":8080"
  forbidden_quoted_key: '"listen-addr" = ":8080"'
related:
  - decision:toml-config-format
  - decision:configbind-supported-types
  - decision:prefix-table-binding
  - rule:toml-shape-validation
  - concept:config-struct-mapping
  - system:configbind
```
