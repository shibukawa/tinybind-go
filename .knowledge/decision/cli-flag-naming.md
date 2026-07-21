---
id: decision:cli-flag-naming
type: decision
title: CLI Flag Naming
---
CLI long flags default to --{prefix}-{field_key}; opt tag replaces defaults and suppresses the prefixed name.

```yaml
status: accepted
default_rule:
  form: '--{prefix}-{key}'
  key_source: intermediate or TOML field key under Bind prefix
  example:
    toml_table: '[webserver]'
    field_key: port
    flag: '--webserver-port'
opt_tag:
  form: 'opt:"long[,short]"'
  meaning: explicit CLI names for this field
  long: first comma-separated token becomes --long
  short: optional second token becomes -short single-letter flag
  suppresses: default '--{prefix}-{key}' is not registered
  examples:
    - 'opt:"port,p" -> --port and -p; no --webserver-port'
    - 'opt:"port" -> --port only; no short; no --webserver-port'
rules:
  - default applies when opt tag is absent
  - when opt is present, only names listed in opt are registered
  - short form is single rune; invalid short is codegen error
  - nested field keys use dotted or flattened key segment per decision:prefix-table-binding
  - intermediate overlay key remains prefix.key regardless of CLI flag rename
  - CLI parse still writes overlay under stable config key, not under flag name alone
process_level_flags:
  - name: config-path
    form: '--config-path'
    role: explicit TOML config file path
    priority: decision:config-file-path-resolution highest over dir search
    not_a_bind_field: true
related:
  - decision:struct-field-tags
  - decision:prefix-table-binding
  - decision:config-file-path-resolution
  - requirement:config-file-discovery
  - requirement:cli-option-codegen
  - data:cli-flag-def
  - concept:cli-option-codegen
  - system:configbind
```
