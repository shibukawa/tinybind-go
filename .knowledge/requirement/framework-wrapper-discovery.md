---
id: requirement:framework-wrapper-discovery
type: requirement
title: Framework Wrapper Discovery
---
Frameworks can wrap every generator-recognized tinybind operation and register wrapper call semantics explicitly.

```yaml
priority: must
scope:
  packages: [httpbind, jsonbind, sqlbind, configbind]
  operations:
    - mapping and validation entry points
    - response and stream writers
    - standalone JSON codecs
    - SQL row scanners
    - config Bind registration
    - route registration
    - error response constructors used by OpenAPI
configuration:
  model: data:generator-call-pattern
  owner: data:generator-options
  registration: api:generator-call-registration
  identity: rule:go-types-symbol-identity
analysis:
  - match the concrete wrapper call site by configured go/types identity
  - read semantic types and values from configured role sources
  - do not traverse, inline, or trust-infer the wrapper function body
  - accept wrapper names unrelated to the underlying tinybind function name
  - accept extra and reordered arguments
  - accept compile-time string and integer constants, not only literal syntax
  - report dynamic values when generation requires a static prefix, route, or status
type_parameter_call_sites:
  case: a wrapper body calls the wrapped operation with its own type parameter
  rule: skip the call site; a type parameter names no concrete config type there
  why_not_an_error:
    - unresolvable and not-a-generation-target are different outcomes
    - one such call currently aborts analysis of the whole package, so unrelated
      config types in that package generate nothing
  where_the_type_is_read: the instantiation site, matched through its own registered call pattern
  keep_as_error: a role index outside the signature, and a concrete type that fails to resolve
consistency:
  - one normalized pattern set drives mapping generation, configbind generation, routes, OpenAPI, checks, and diagnostics
  - configbind analysis receives the same generator options as other analyzers
  - operation meaning comes from the registered pattern, never from function name switching
runtime_contract:
  - framework wrapper implements behavior compatible with its declared operation
  - generator does not import the framework package at runtime
acceptance:
  - RegisterConfig[ServerConfig](ctx, "server") emits the ServerConfig config definition when configured
  - the wrapper may place prefix at any declared argument index
  - two calls of one wrapper with different concrete types or static prefixes generate independent targets
  - an unconfigured same-named function is ignored
  - structurally invalid role mapping fails registration or option normalization with an actionable diagnostic
  - a role index outside a resolved wrapper signature fails package analysis with an actionable diagnostic
  - a framework command registers its wrapper catalog once and reuses the resulting immutable options across packages
  - direct tinybind calls are represented by default call patterns, not special analyzer branches
  - 'a generic helper calling configbind.Bind[T](prefix) is skipped, and other config types in that package still generate'
  - the same package holding both the generic helper and its own config types needs no package split
related:
  - requirement:alias-transparent-type-analysis
  - requirement:configurable-generator-discovery
  - data:generator-call-pattern
  - data:generator-options
  - rule:go-types-symbol-identity
  - flow:code-generation
  - flow:configbind-codegen
  - api:generator-call-registration
```
