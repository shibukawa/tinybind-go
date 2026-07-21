---
id: decision:configbind-supported-types
type: decision
title: configbind Supported Field Types
---
v1 field types target web-server options; primitives, primitive arrays, and nested structs are allowed.

```yaml
status: accepted
audience: web server / service CLI options
supported_scalars:
  - bool
  - int
  - string
  - duration
  - datetime
  - url
supported_composites:
  - array of supported scalars only
  - nested struct fields mapped to nested TOML tables
go_type_hints:
  bool: bool
  int: int or sized integer TBD
  string: string
  duration: time.Duration
  datetime: time.Time
  url: net/url.URL or string-parsed URL type TBD
  array: '[]T where T is a supported scalar'
  nested_struct: named Go struct fields
out_of_scope_v1:
  - file paths as first-class config value types
  - multipart or file upload handling
  - binary blobs
  - arbitrary nested maps of mixed types
  - arrays of structs
  - arrays of tables in TOML
  - inline tables in TOML
  - quoted keys in TOML
toml_shape: decision:toml-shape-constraints
rationale:
  - configbind is an option parser for services, not a general file binder
  - primitive arrays cover multi-value flags such as origins or tags
  - nested structs use standard tables, not inline or table arrays
  - smaller shape simplifies codegen and TinyGo portability
related:
  - requirement:configbind-product-goals
  - requirement:configbind-tinygo
  - concept:config-struct-mapping
  - decision:toml-shape-constraints
  - rule:toml-shape-validation
  - system:configbind
```
