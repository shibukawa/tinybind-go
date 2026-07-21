---
id: requirement:struct-registration
type: requirement
title: Config Struct Registration
---
Users bind any number of Go structs via Bind[T](prefix); each prefix owns a TOML table and matching env/CLI namespace.

```yaml
priority: must
api: api:configbind-bind
behavior:
  - call configbind.Bind[T](prefix) zero or more times before load
  - each Bind associates type T with prefix
  - TOML values for T come from table [prefix]
  - load maps keys under each prefix onto the bound target
  - nested structs map under the same prefix hierarchy via nested tables
constraints:
  - prefixes partition the config key space
  - registration order must not change precedence of sources
  - unregistered keys may be ignored or reported per policy TBD
  - type mismatches on mapped fields are errors
  - field types limited by decision:configbind-supported-types
  - TOML shape limited by decision:toml-shape-constraints
example:
  go: 'c := configbind.Bind[WebServiceConfig]("webservice")'
  toml_table: "[webservice]"
  nested_table: "[webservice.tls]"
related:
  - decision:prefix-table-binding
  - decision:toml-shape-constraints
  - concept:config-struct-mapping
  - system:configbind
  - flow:config-load
  - requirement:configbind-product-goals
acceptance:
  - two Bind calls with different prefixes receive disjoint table namespaces
  - Bind[WebServiceConfig]("webservice") only reads [webservice] and nested tables
  - one load call updates every bound target consistently
```
