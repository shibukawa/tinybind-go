---
id: flow:configbind-codegen
type: flow
title: configbind Codegen Pipeline
---
Generator reads Bind and SubCommand AST usage and emits reflection-free apply, CLI wiring, key tables, and scaffold text.

```yaml
flow:
  trigger: developer runs configbind generator or go generate
  architecture: decision:configbind-codegen-no-reflect
  steps:
    - id: discover-structs
      action: find types from api:configbind-bind and api:configbind-subcommand usage
      refs:
        - requirement:struct-registration
        - api:configbind-bind
        - api:configbind-subcommand
        - decision:prefix-table-binding
    - id: parse-fields
      action: read fields, supported types, and default|help|opt|enum|secret|arg tags at compile time
      refs:
        - decision:configbind-supported-types
        - decision:struct-field-tags
        - decision:cli-flag-naming
        - requirement:struct-field-metadata
        - rule:enum-value-validation
        - rule:secret-redaction
    - id: build-ir
      action: build IR including data:cli-flag-def list, Bind options, overlay keys, subcommands, scaffolds
      refs:
        - concept:config-overlay
        - data:cli-flag-def
        - decision:cli-flag-naming
        - decision:configbind-runtime-architecture
    - id: emit-apply
      action: generate overlay-to-struct apply and default seeding without reflection
      refs:
        - concept:config-struct-mapping
        - concept:config-overlay
        - decision:configbind-codegen-no-reflect
    - id: emit-cli-parser
      action: emit flags from data:cli-flag-def; default --prefix-key or opt long/short; SubCommand *T or nil
      refs:
        - concept:cli-option-codegen
        - concept:reusable-source-parsers
        - requirement:cli-option-codegen
        - requirement:cli-subcommands
        - concept:subcommand-binding
        - api:configbind-subcommand
        - decision:cli-flag-naming
        - data:cli-flag-def
    - id: emit-key-tables
      action: generate known env/CLI/TOML key lists for filters and provenance
      refs:
        - concept:config-overlay
        - term:config-key
    - id: emit-scaffold-subcommands
      action: embed TOML and .env with help comments; print to stdout
      refs:
        - requirement:scaffold-generation
        - concept:scaffold-templates
        - data:cli-flag-def
  related:
    - system:configbind
    - flow:config-load
    - requirement:configbind-tinygo
    - decision:configbind-runtime-architecture
```
