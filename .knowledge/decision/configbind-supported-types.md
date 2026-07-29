---
id: decision:configbind-supported-types
type: decision
title: configbind Supported Field Types
---
v1 field types target web-server options; primitives, primitive arrays, nested structs, and struct slices are allowed.

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
  - slice of a same-package named struct, mapped to a TOML array of tables
go_type_hints:
  bool: bool
  int: int or sized integer TBD
  string: string
  duration: time.Duration
  datetime: time.Time
  url: net/url.URL or string-parsed URL type TBD
  array: '[]T where T is a supported scalar'
  nested_struct: named Go struct fields
  struct_slice: '[]T where T is a named struct in the same package; []*T is rejected'
codegen_field_kinds:
  - FieldString
  - FieldBool
  - FieldInt
  - FieldDuration
  - FieldStringSlice
  - FieldStruct
  - FieldStructSlice
duration:
  requirement: requirement:duration-config-fields
  value_form: rule:duration-value-parsing
  detection_hazard: >
    time.Duration has underlying int64, so kind detection must match the named
    type first or duration silently binds as int
  array_of_duration: out of scope in v1
out_of_scope_v1:
  - file paths as first-class config value types
  - multipart or file upload handling
  - binary blobs
  - arbitrary nested maps of mixed types
  - flags or env vars for array-of-tables elements
  - recursive config structs
  - inline tables in TOML
  - arrays of inline tables in TOML
  - quoted keys in TOML
toml_shape: decision:toml-shape-constraints
rationale:
  - configbind is an option parser for services, not a general file binder
  - primitive arrays cover multi-value flags such as origins or tags
  - nested structs use standard tables, never inline tables
  - repeated settings are data, so struct slices read from arrays of tables
  - an element count has no CLI or env form, so those layers skip struct slices
  - smaller shape simplifies codegen and TinyGo portability
related:
  - requirement:configbind-product-goals
  - requirement:configbind-tinygo
  - requirement:duration-config-fields
  - rule:duration-value-parsing
  - concept:config-struct-mapping
  - decision:toml-shape-constraints
  - rule:toml-shape-validation
  - system:configbind
```
