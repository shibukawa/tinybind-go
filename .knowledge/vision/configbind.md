---
id: vision:configbind
type: vision
title: configbind Vision
---
configbind loads web-server options from TOML, environment variables, and CLI flags into prefixed Go structs with generated parsers, subcommands, scaffolds, provenance, and TinyGo support.

```yaml
source_of_truth:
  - developer-defined Go config structs and field tags
generated_from_types:
  - CLI option parsers
  - subcommand dispatch and positional arg parsers
  - field keys and env names under Bind prefixes
  - TOML and .env scaffold templates
  - overlay-to-struct apply helpers without reflection
principles:
  - multi-source load with later-wins precedence
  - reusable source parsers produce maps; configbind merges overlay
  - generated apply only; no runtime reflection
  - N structs via Bind[T](prefix) with TOML table partitioning
  - defaults and help on struct tags
  - SubCommand returns *T or nil; CLI-only
  - TOML and env for shared Bind config only
  - generated scaffold fragments aggregate across imported packages; printing commands remain application-owned
  - TOML as config file format
  - provenance log helper with secret redaction for observability
  - TinyGo first-class
  - service option types only, not file binding
  - restricted TOML shape: bare keys, dotted keys, nested tables, primitive arrays
targets:
  - system:configbind
  - requirement:configbind-product-goals
  - requirement:configbind-tinygo
  - requirement:struct-field-metadata
  - requirement:scaffold-generation
  - requirement:cli-subcommands
  - concept:config-struct-mapping
  - concept:layered-config-sources
  - concept:cli-option-codegen
  - concept:provenance-callback
  - concept:subcommand-binding
  - concept:scaffold-templates
  - concept:config-overlay
  - concept:reusable-source-parsers
  - api:configbind-bind
  - api:configbind-subcommand
  - decision:prefix-table-binding
  - decision:configbind-supported-types
  - decision:toml-shape-constraints
  - decision:struct-field-tags
  - decision:configbind-runtime-architecture
  - decision:configbind-codegen-no-reflect
```
