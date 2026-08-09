---
id: requirement:configurable-generator-discovery
type: requirement
title: Configurable Generator Discovery
---
Applications provide the complete route, runtime, configbind, and wrapper discovery set through one option used by api:generator-execution and api:generator-main.

```yaml
status: required
configuration:
  call_binding: data:generator-call-pattern
  method_identity: package path + method name + receiver package path + receiver type
  special_type_identity: package path + type name + generator role
  model: data:generator-options
defaults:
  calls: Bind, Write, WriteStatus, DecodeJSON, EncodeJSON, WriteStream, ScanRows, configbind.Bind, error constructors
  routes: net/http package registrars and net/http.ServeMux methods
  special_types: httpbind.File
behavior:
  - Set is the complete identity set for that field
  - configured sets never retain hidden built-in identities
  - use DefaultOptions explicitly when standard identities are wanted
  - Disabled suppresses discovery and generation for that operation
  - RuntimePackages may expand canonical same-named call layouts as shorthand
  - arbitrary framework wrappers use explicit semantic operation and role sources
  - aliases resolve through rule:go-types-symbol-identity
  - same-named unconfigured symbols never match
  - duplicate Set identities normalize to one match
  - configbind and OpenAPI use the same normalized call patterns as mapping generation
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
  - requirement:framework-wrapper-discovery
  - data:generator-call-pattern
  - api:generator-execution
```
