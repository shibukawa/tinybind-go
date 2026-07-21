---
id: requirement:cli-option-codegen
type: requirement
title: CLI Option Code Generation
---
Code generation emits CLI option and subcommand parser code from registered struct definitions and tags.

```yaml
priority: must
intent: avoid hand-written flag definitions that drift from struct fields
audience: web server / service option flags and subcommands
generated_artifacts:
  - CLI flag definitions per field as data:cli-flag-def IR then emitted code
  - subcommand dispatch from api:configbind-subcommand
  - positional arg parsers from arg tags
  - parse function over os.Args or flag set
  - help/usage text from help tags
inputs:
  - Go config struct types from api:configbind-bind
  - subcommand types from api:configbind-subcommand
  - prefix string per Bind
  - decision:struct-field-tags including opt and help
  - decision:cli-flag-naming
  - decision:configbind-supported-types only
  - nested struct fields and primitive array fields
outputs:
  - Go source wiring flags into maps/overlay and SubCommand *T
  - no reflection-based flag discovery
  - struct-derived flag table extractable at generate time as data:cli-flag-def list
flag_naming: decision:cli-flag-naming
naming_examples:
  - '[webserver] port without opt -> --webserver-port'
  - 'opt:"port,p" -> --port and -p; omit --webserver-port'
process_flags:
  - '--config-path <path>': explicit config file; decision:config-file-path-resolution
integration:
  - Bind CLI values merge into concept:config-overlay under stable config_key
  - flag rename via opt does not rename overlay key
  - concept:cli-option-codegen
non_goals:
  - file path or file-content CLI option types
  - arrays of structs
  - runtime reflection
related:
  - concept:cli-option-codegen
  - concept:subcommand-binding
  - concept:reusable-source-parsers
  - concept:config-overlay
  - data:cli-flag-def
  - flow:configbind-codegen
  - requirement:layered-config-load
  - requirement:cli-subcommands
  - requirement:struct-field-metadata
  - requirement:scaffold-generation
  - decision:prefix-table-binding
  - decision:configbind-supported-types
  - decision:toml-shape-constraints
  - decision:struct-field-tags
  - decision:cli-flag-naming
  - decision:configbind-codegen-no-reflect
  - decision:configbind-runtime-architecture
  - requirement:configbind-tinygo
  - system:configbind
acceptance:
  - generator extracts data:cli-flag-def list from struct fields and tags
  - default flag for Bind prefix webserver field port is --webserver-port
  - opt:"port,p" registers --port and -p and does not register --webserver-port
  - help tags appear in generated CLI usage
  - regenerating after field rename updates default flag names
  - CLI layer can override TOML and env for the same config_key
  - primitive multi-value flags map to array fields
  - default tags seed default layer before CLI override
  - enum tags appear in usage and reject invalid CLI values
  - SubCommand returns *T when selected and nil otherwise
  - SubCommand fields are CLI-only and never loaded from TOML or env
  - generated CLI path does not import reflect for binding
```
