---
id: flow:configbind-codegen
type: flow
title: configbind Codegen Pipeline
---
Generator reads one package's Bind and SubCommand usage and emits reflection-free apply, CLI wiring, key tables, and scaffold fragment registrations.

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
    - id: emit-scaffold-fragments
      action: register data:config-scaffold-fragment per Bind type and prefix; api:config-scaffold-output renders all imported package fragments
      refs:
        - requirement:scaffold-generation
        - requirement:modular-package-generation
        - concept:scaffold-templates
        - data:config-scaffold-fragment
        - data:cli-flag-def
  related:
    - system:configbind
    - flow:config-load
    - requirement:configbind-tinygo
    - decision:configbind-runtime-architecture
```
