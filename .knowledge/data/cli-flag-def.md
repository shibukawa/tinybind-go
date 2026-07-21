---
id: data:cli-flag-def
type: data
title: CLI Flag Definition
---
Codegen IR entry describing one CLI flag derived from a Bind or SubCommand struct field.

```yaml
fields:
  - name: config_key
    type: string
    ref: term:config-key
    description: stable overlay key e.g. webserver.port
  - name: long_flags
    type: '[]string'
    description: long names without leading dashes e.g. webserver-port or port
  - name: short_flags
    type: '[]string'
    description: optional single-letter short names e.g. p
  - name: help
    type: string
    description: from help tag; CLI usage and scaffold comments
  - name: default
    type: string
    optional: true
    description: from default tag
  - name: enum
    type: '[]string'
    optional: true
  - name: field_type
    type: string
    description: supported scalar or primitive array kind
  - name: uses_opt_override
    type: bool
    description: true when opt tag replaced default prefix naming
derivation:
  - generator reads Go struct AST and field tags at generate time
  - default long flag from decision:cli-flag-naming when opt absent
  - opt tag populates long_flags and short_flags and sets uses_opt_override
  - help tag fills help for CLI and TOML scaffold comments
used_by:
  - concept:cli-option-codegen
  - requirement:cli-option-codegen
  - requirement:scaffold-generation
  - flow:configbind-codegen
related:
  - decision:struct-field-tags
  - decision:cli-flag-naming
  - decision:configbind-codegen-no-reflect
```
