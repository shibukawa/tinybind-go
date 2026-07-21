---
id: api:configbind-bind
type: api
title: configbind.Bind
---
Generic Bind registers a config struct type under a string prefix and returns a handle for load results.

```yaml
signature_sketch: 'func Bind[T any](prefix string) *Binding[T]'
behavior:
  - associates T with TOML table [prefix]
  - participates in multi-source load under that prefix
  - returns typed handle to access final T after load
prefix_rules:
  - non-empty string
  - becomes TOML table name
  - prefixes partition config space across structs
example:
  - 'c := configbind.Bind[WebServiceConfig]("webservice")'
depends_on:
  - decision:prefix-table-binding
  - requirement:struct-registration
  - concept:config-struct-mapping
related:
  - system:configbind
  - flow:config-load
  - decision:configbind-supported-types
```
