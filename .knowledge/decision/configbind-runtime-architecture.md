---
id: decision:configbind-runtime-architecture
type: decision
title: configbind Runtime Architecture
---
Reusable source parsers feed a typed overlay; generated code applies overlay keys onto structs without reflection.

```yaml
status: accepted
layers:
  - id: reusable_source_parsers
    role: domain-agnostic parsers and tokenizers
    examples:
      - restricted TOML document to flat or nested key/value map
      - CLI argv tokenizer / flag token stream to key/value map
      - env snapshot reader
      - config file path discovery via system:configdir and --config-path
    constraint: not tied to a specific application config struct
    concept: concept:reusable-source-parsers
  - id: overlay_merge
    role: key-wise multi-source merge with provenance Place
    data: data:overlay-entry
    concept: concept:config-overlay
    precedence: rule:source-precedence
  - id: generated_bind
    role: type-specific apply, defaults, enum checks, CLI flag wiring, scaffolds
    constraint: no runtime reflection
    decision: decision:configbind-codegen-no-reflect
  - id: public_configbind_api
    role: Bind, SubCommand, Load, Provenance integration
    system: system:configbind
pipeline:
  - resolve single TOML path via decision:config-file-path-resolution
  - source parsers produce key/value pairs (generic maps or streams)
  - configbind merges into concept:config-overlay with Place
  - generated apply writes supported types into Bind targets
  - SubCommand fills *T from CLI only, outside TOML/env overlay
  - Provenance builds []{Key, Value, Place} from overlay plus secret policy
non_goals:
  - reflection-based map-to-struct core
  - requiring source parsers to know Go struct layouts
related:
  - concept:reusable-source-parsers
  - concept:config-overlay
  - concept:config-struct-mapping
  - flow:config-load
  - flow:configbind-codegen
  - requirement:layered-config-load
  - requirement:config-file-discovery
  - decision:config-file-path-resolution
  - system:configdir
  - requirement:configbind-tinygo
  - system:configbind
```
