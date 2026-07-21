---
id: term:config-source
type: term
title: Config Source
---
Origin layer that supplied a config value during layered load.

```yaml
definition: named layer in the config precedence chain
values:
  - default
  - file_toml
  - env
  - cli
log_field: Place in data:provenance-event
related:
  - rule:source-precedence
  - concept:layered-config-sources
  - data:provenance-event
  - concept:provenance-log-helper
```
