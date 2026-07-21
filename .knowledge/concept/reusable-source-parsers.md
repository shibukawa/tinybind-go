---
id: concept:reusable-source-parsers
type: concept
title: Reusable Source Parsers
---
Domain-agnostic parsers convert TOML, CLI argv, and env into key/value maps without knowing application structs.

```yaml
meaning_of_generic:
  - not limited to one app config schema
  - reusable outside or beside configbind integration
  - still may enforce configbind TOML shape rules when used by configbind
components:
  toml:
    input: TOML text or single resolved file path
    output: map of bare key paths to scalar or primitive-array values
    constraints: decision:toml-shape-constraints
    path: decision:config-file-path-resolution
  cli:
    input: argv tokens
    output: map of flag keys to raw string values plus positional tokens
    note: flag names for an app may be generated; the token/map machinery stays generic
    process_flags:
      - '--config-path' for explicit config file path
  env:
    input: process environment
    output: map of env names to string values
  config_dirs:
    library: system:configdir
    input: vendor_name, tool_name, and file_name from API
    output: one directory/file path when present
integration:
  - configbind consumes parser outputs into concept:config-overlay
  - generated code supplies known key sets and apply logic
  - parsers do not reflect on Go types
  - path discovery chooses one file before TOML parse
related:
  - decision:configbind-runtime-architecture
  - decision:configbind-codegen-no-reflect
  - decision:config-file-path-resolution
  - requirement:config-file-discovery
  - system:configdir
  - concept:config-overlay
  - flow:config-load
  - system:configbind
```
