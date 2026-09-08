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
  - 'file_env:<file>; the env layer's place when a LoadOptions.EnvFiles entry supplied the name, per rule:env-file-composition'
  - cli
log_field: Place in data:provenance-event
decode: EnvFileOf in api:configbind-env-files returns the file a file_env place names
related:
  - rule:source-precedence
  - concept:layered-config-sources
  - data:provenance-event
  - concept:provenance-log-helper
```
