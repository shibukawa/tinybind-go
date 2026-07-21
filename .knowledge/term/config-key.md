---
id: term:config-key
type: term
title: Config Key
---
Stable path identifying one setting under a Bind prefix across TOML, env, CLI, and struct fields.

```yaml
definition: hierarchical identifier for a single configuration value within a prefix namespace
forms:
  - TOML path under table [prefix]
  - env variable name including prefix
  - default CLI flag --prefix-key unless opt renames flags
  - Go struct field path on the bound type
notes:
  - opt CLI aliases do not change the stable config key used in overlay and TOML
example:
  - prefix webserver + field port -> config key webserver.port; default CLI --webserver-port
  - opt:"port,p" still maps CLI values to webserver.port
related:
  - term:config-source
  - data:provenance-event
  - data:cli-flag-def
  - concept:config-struct-mapping
  - decision:prefix-table-binding
  - decision:cli-flag-naming
  - api:configbind-bind
```
