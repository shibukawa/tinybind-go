---
id: data:generator-options
type: data
title: Generator Discovery Options
---
Generator options describe complete semantic call and type discovery sets without importing target symbols.

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
  CallPattern:
    model: data:generator-call-pattern
    use: function or method identity plus semantic type and argument role sources
pattern_set:
  shape: "PatternSet[T] { Set []T; Disabled bool }"
  precedence:
    - Disabled yields empty set
    - non-nil Set is authoritative, including an explicitly empty slice
    - nil Set contains no identities unless the operation inherits explicit RuntimePackages expansion
construction:
  direct: construct complete sets as values
  framework: api:generator-call-registration adds wrapper patterns and returns an immutable Options snapshot
options:
  ServeMuxes: TypePattern set expanded to Handle and HandleFunc methods
  RouteMethods: MethodPattern set for nonstandard registration method names
  RouteFunctions: SymbolPattern set for package registration functions
  Calls: CallPattern set for every generator-recognized operation
  RuntimePackages: optional package-path shorthand expanded to canonical same-named CallPatterns
  FileTypes: TypePattern set
  HTMLTemplatePattern: base-name glob; empty uses '*.tb.html'
  SQLTemplatePattern: base-name glob; empty uses '*.tb.sql'
  SQLContextAPI: bool; opt in to decision:sql-context-executor-api wrappers
  SQLContextOnlyAPI: bool; decision:sql-context-executor-api context-only public surface
  SQLExecutorResolver: optional SymbolPattern; framework resolver that implies SQLContextAPI
  DisableFeatures: rule:generator-feature-disable
runtime_package_expansion:
  functions: [Bind, Write, WriteStatus, DecodeJSON, EncodeJSON, NewStream, ScanRows]
  rule: non-nil Calls.Set replaces all RuntimePackages expansion; CallRegistry.Options merges base expansion and registered wrappers into one explicit Calls snapshot
wrapper_package:
  arbitrary_name: explicit data:generator-call-pattern
  added_or_reordered_arguments: explicit role sources
  fixed_semantics: constant role sources
  runtime_contract: requirement:framework-wrapper-discovery
default_options:
  constructor: DefaultOptions
  ServeMuxes: [net/http.ServeMux]
  RouteFunctions: [net/http.Handle, net/http.HandleFunc]
  RuntimePackages:
    - github.com/shibukawa/tinybind-go
    - github.com/shibukawa/tinybind-go/jsonbind
    - github.com/shibukawa/tinybind-go/sqlbind
  FileTypes: [github.com/shibukawa/tinybind-go.File]
  HTMLTemplatePattern: '*.tb.html'
  SQLTemplatePattern: '*.tb.sql'
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
  - data:generator-call-pattern
  - requirement:framework-wrapper-discovery
  - api:generator-call-registration
  - requirement:configurable-template-file-patterns
  - decision:sql-context-executor-api
  - requirement:custom-framework-generation-profile
```
