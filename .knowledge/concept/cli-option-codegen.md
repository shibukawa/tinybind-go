---
id: concept:cli-option-codegen
type: concept
title: CLI Option Codegen
---
Generator derives data:cli-flag-def from Bind structs and emits type-safe CLI flag parsing code.

```yaml
artifacts:
  - data:cli-flag-def IR per option field
  - flag registration using decision:cli-flag-naming
  - subcommand dispatch returning *T or nil
  - positional arg parsers for SubCommand only
  - parse entrypoint
  - help strings from help tags
  - defaults from default tags
  - scaffold subcommands that print embedded plain text to stdout
  - wiring into concept:reusable-source-parsers CLI map helpers
  - overlay writes for Bind CLI keys under stable config_key
flag_naming:
  default: '--{prefix}-{key}'
  opt_override: 'opt:"long[,short]" replaces default; suppresses prefixed name'
field_types: decision:configbind-supported-types
field_tags: decision:struct-field-tags
feeds_layer: cli
feeds_overlay: concept:config-overlay
pipeline: flow:configbind-codegen
no_reflect: decision:configbind-codegen-no-reflect
related:
  - requirement:cli-option-codegen
  - requirement:cli-subcommands
  - requirement:struct-field-metadata
  - data:cli-flag-def
  - decision:cli-flag-naming
  - api:configbind-bind
  - api:configbind-subcommand
  - decision:prefix-table-binding
  - decision:configbind-runtime-architecture
  - concept:subcommand-binding
  - concept:reusable-source-parsers
  - system:configbind
  - concept:config-struct-mapping
  - requirement:configbind-tinygo
```
