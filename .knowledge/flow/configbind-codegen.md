---
id: flow:configbind-codegen
type: flow
title: configbind Codegen Pipeline
---
Generator reads one package's Bind and SubCommand usage and emits reflection-free Definition and SubCommandDefinition registrations with typed apply code.

```yaml
flow:
  trigger: developer runs configbind generator or go generate
  architecture: decision:configbind-codegen-no-reflect
  steps:
    - id: discover-structs
      action: find concrete types, prefixes, names, and help values through configured data:generator-call-pattern roles for api:configbind-bind and api:configbind-subcommand semantics
      refs:
        - requirement:struct-registration
        - api:configbind-bind
        - api:configbind-subcommand
        - decision:prefix-table-binding
        - requirement:framework-wrapper-discovery
        - api:generator-call-registration
    - id: read-doc-comments
      action: correlate go/types struct fields with AST fields to recover struct and field godoc text
      refs:
        - requirement:godoc-config-descriptions
        - rule:godoc-comment-normalization
        - decision:godoc-help-precedence
    - id: backfill-help
      action: write help:"..." into source struct tags for fields lacking one, then re-read tags
      refs:
        - rule:help-tag-backfill
        - decision:godoc-help-precedence
        - decision:struct-field-tags
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
    - id: emit-definitions
      action: register one configbind Definition per Bind type and prefix plus one SubCommandDefinition per CLI branch; Definition carries Doc from struct godoc; api:config-scaffold-output renders only Bind fields
      refs:
        - requirement:scaffold-generation
        - requirement:godoc-config-descriptions
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
