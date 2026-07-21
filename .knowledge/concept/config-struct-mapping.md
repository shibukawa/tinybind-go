---
id: concept:config-struct-mapping
type: concept
title: Config Struct Mapping
---
Generated code maps overlay keys under a Bind prefix onto fields of the bound Go struct without reflection.

```yaml
behavior:
  - Bind[T](prefix) scopes all keys to TOML table [prefix]
  - generator emits apply functions for T fields
  - apply reads concept:config-overlay entries and parses decision:configbind-supported-types
  - nested Go structs map to nested standard tables or dotted bare keys under the prefix
  - default, help, enum, secret tags resolved at generation time via decision:struct-field-tags
  - key paths under prefix align with TOML hierarchy and prefixed env/CLI names
  - multiple structs use prefix partitioning, not a shared flat key space
  - reject shapes forbidden by decision:toml-shape-constraints
  - no runtime reflection for assignment or tag reading
depends_on:
  - requirement:struct-registration
  - decision:prefix-table-binding
  - decision:configbind-supported-types
  - decision:toml-shape-constraints
  - decision:struct-field-tags
  - decision:configbind-codegen-no-reflect
  - decision:configbind-runtime-architecture
  - concept:config-overlay
  - rule:toml-shape-validation
  - flow:config-load
related:
  - api:configbind-bind
  - system:configbind
  - concept:layered-config-sources
  - concept:reusable-source-parsers
  - term:config-key
  - requirement:struct-field-metadata
example:
  - 'configbind.Bind[WebServiceConfig]("webservice") -> [webservice] table'
  - nested field TLS maps to [webservice.tls] or webservice.tls.x dotted keys
  - []string field maps to TOML array of strings
```
