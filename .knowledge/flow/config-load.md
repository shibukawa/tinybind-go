---
id: flow:config-load
type: flow
title: Config Load Pipeline
---
Parse reusable sources into maps, merge into overlay with Place, apply via generated code, resolve SubCommand *T, emit provenance.

```yaml
flow:
  trigger: application calls load after Bind and SubCommand registrations
  architecture: decision:configbind-runtime-architecture
  steps:
    - id: bind-structs
      action: accept N Bind[T](prefix) registrations for shared config
      refs:
        - api:configbind-bind
        - requirement:struct-registration
        - decision:prefix-table-binding
        - concept:config-struct-mapping
    - id: register-subcommands
      action: accept SubCommand[T](name, help) registrations as CLI-only branches
      refs:
        - api:configbind-subcommand
        - requirement:cli-subcommands
        - concept:subcommand-binding
    - id: seed-defaults
      action: generated defaults write known keys into concept:config-overlay with place default
      refs:
        - concept:config-overlay
        - concept:layered-config-sources
        - decision:struct-field-tags
        - requirement:struct-field-metadata
        - decision:configbind-codegen-no-reflect
    - id: resolve-config-path
      action: choose one TOML path via --config-path or configdir user-then-system for vendor_name, tool_name, file_name
      refs:
        - decision:config-file-path-resolution
        - requirement:config-file-discovery
        - system:configdir
        - decision:cli-flag-naming
      notes:
        - vendor_name and tool_name come from public API
        - --config-path unreadable yields error without fallback
    - id: parse-cli-early-path
      action: parse process flags including --config-path before TOML load when needed
      refs:
        - concept:reusable-source-parsers
        - concept:cli-option-codegen
        - requirement:config-file-discovery
    - id: parse-toml
      action: parse the single resolved TOML file into key/value map; merge as file_toml; no multi-file merge
      refs:
        - concept:reusable-source-parsers
        - concept:config-overlay
        - decision:toml-config-format
        - decision:prefix-table-binding
        - decision:toml-shape-constraints
        - rule:toml-shape-validation
        - decision:config-file-path-resolution
        - api:configbind-bind
    - id: parse-env
      action: reusable env reader yields map; generated known-key filter merges as env
      refs:
        - concept:reusable-source-parsers
        - concept:config-overlay
        - api:configbind-bind
    - id: parse-cli
      action: generic CLI map machinery plus generated flag names merge Bind keys as cli; dispatch SubCommand to *T or nil
      refs:
        - concept:reusable-source-parsers
        - concept:cli-option-codegen
        - concept:config-overlay
        - requirement:cli-option-codegen
        - requirement:cli-subcommands
        - decision:struct-field-tags
        - api:configbind-subcommand
    - id: apply-structs
      action: generated apply maps overlay onto Bind structs with typed parse and enum checks
      refs:
        - concept:config-struct-mapping
        - concept:config-overlay
        - decision:configbind-supported-types
        - decision:configbind-codegen-no-reflect
        - rule:enum-value-validation
        - concept:subcommand-binding
    - id: emit-provenance
      action: build redacted []{Key, Value, Place} from overlay via log helper
      refs:
        - concept:provenance-log-helper
        - concept:provenance-callback
        - concept:config-overlay
        - data:provenance-event
        - data:overlay-entry
        - requirement:source-provenance-logging
        - rule:secret-redaction
        - decision:struct-field-tags
  invariant: later overlay Set wins per key via rule:source-precedence; no runtime reflection
  related:
    - system:configbind
    - requirement:layered-config-load
    - requirement:config-file-discovery
    - requirement:configbind-tinygo
    - decision:configbind-runtime-architecture
    - system:configdir
```
