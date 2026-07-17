---
id: data:generator-options
type: data
title: Generator Discovery Options
---
Generator options describe complete sets of exact go/types identities without importing target symbols.

```yaml
status: required
identity_types:
  SymbolPattern:
    fields: [PackagePath, Name]
    use: package function
  TypePattern:
    fields: [PackagePath, Name]
    use: named receiver or special type
  MethodPattern:
    fields: [PackagePath, Name, ReceiverPackagePath, ReceiverType]
    use: exact method identity
pattern_set:
  shape: "PatternSet[T] { Set []T; Disabled bool }"
  precedence:
    - Disabled yields empty set
    - non-nil Set is authoritative, including an explicitly empty slice
    - nil Set contains no identities unless the operation inherits explicit RuntimePackages expansion
options:
  ServeMuxes: TypePattern set expanded to Handle and HandleFunc methods
  RouteMethods: MethodPattern set for nonstandard registration method names
  RouteFunctions: SymbolPattern set for package registration functions
  RuntimePackages: package-path set expanded to same-named enabled runtime operations
  Bind: SymbolPattern set
  Write: SymbolPattern set
  WriteStatus: SymbolPattern set
  DecodeJSON: SymbolPattern set
  EncodeJSON: SymbolPattern set
  NewStream: SymbolPattern set
  ScanRows: SymbolPattern set
  FileTypes: TypePattern set
  DisableFeatures: rule:generator-feature-disable
runtime_package_expansion:
  functions: [Bind, Write, WriteStatus, DecodeJSON, EncodeJSON, NewStream, ScanRows]
  rule: non-nil operation-specific Set replaces expansion for that operation
compatibility_package:
  type_alias: accepted for special types when configured by alias package identity
  function_export: declare same-named forwarding generic functions; Go has no generic function alias
  runtime_contract: forwarding functions must preserve httpbinder-compatible signatures and semantics
default_options:
  constructor: DefaultOptions
  ServeMuxes: [net/http.ServeMux]
  RouteFunctions: [net/http.Handle, net/http.HandleFunc]
  RuntimePackages: [github.com/shibukawa/httpbind-go]
  FileTypes: [github.com/shibukawa/httpbind-go.File]
zero_options: no discovery identities; CLI capabilities remain subject to rule:generator-feature-disable
identity_reason:
  use: package import path plus declared name
  avoid_reflect:
    - reflect values require importing optional target packages into the custom command
    - generic functions cannot be represented uniformly as function values
    - go/types already resolves aliases and receiver identity on the host
petitweb:
  serve_mux: github.com/shibukawa/petitweb-go/handler.ServeMux
  runtime_package: github.com/shibukawa/petitweb-go/handler
related:
  - api:generator-main
  - requirement:configurable-generator-discovery
  - rule:go-types-symbol-identity
  - rule:generator-feature-disable
```
