---
id: requirement:configurable-generator-discovery
type: requirement
title: Configurable Generator Discovery
---
Applications provide the complete route and runtime discovery set through one generator option passed to api:generator-main.

```yaml
status: required
configuration:
  call_identity: package path + function name
  method_identity: package path + method name + receiver package path + receiver type
  special_type_identity: package path + type name + generator role
  model: data:generator-options
defaults:
  calls: Bind, Write, WriteStatus, DecodeJSON, EncodeJSON, NewStream
  routes: net/http package registrars and net/http.ServeMux methods
  special_types: httpbinder.File
behavior:
  - Set is the complete identity set for that field
  - configured sets never retain hidden built-in identities
  - use DefaultOptions explicitly when standard identities are wanted
  - Disabled suppresses discovery and generation for that operation
  - additional runtime package expands same-named runtime functions
  - aliases resolve through rule:go-types-symbol-identity
  - same-named unconfigured symbols never match
  - duplicate Set identities normalize to one match
public_surface:
  - api:generator-main
  - reusable configured generator object
  - configured package analyzer
  - configured route parser
host_only: true
related:
  - requirement:strict-symbol-identity
  - rule:go-types-symbol-identity
  - concept:code-generation
  - flow:code-generation
  - data:generator-options
  - rule:generator-feature-disable
```
