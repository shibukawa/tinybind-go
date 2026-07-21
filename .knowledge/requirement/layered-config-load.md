---
id: requirement:layered-config-load
type: requirement
title: Layered Config Load
---
Load shared Bind settings through reusable parsers into an overlay; later layers override earlier keys. Subcommands stay CLI-only.

```yaml
priority: must
applies_to: api:configbind-bind shared config only
does_not_apply_to: api:configbind-subcommand
architecture: decision:configbind-runtime-architecture
layers:
  - id: default
    description: generated default tag values into concept:config-overlay
    tags: decision:struct-field-tags
  - id: file
    description: single resolved TOML file map merged as file_toml
    format: decision:toml-config-format
    path: decision:config-file-path-resolution
    parser: concept:reusable-source-parsers
  - id: env
    description: reusable env map filtered by generated known keys
    parser: concept:reusable-source-parsers
  - id: cli
    description: CLI key/value map for Bind fields via generated names plus generic machinery
    parser: concept:reusable-source-parsers
precedence: rule:source-precedence
rules:
  - only set keys override; absent keys leave prior overlay entry
  - CLI is highest priority among external sources for Bind fields
  - defaults apply only when no external source sets the key
  - keys are scoped by Bind prefix / TOML table
  - SubCommand fields never read TOML or env and are not overlay-backed
  - struct apply is generated; no runtime reflection
  - only one TOML file is loaded; user and system files are not merged
related:
  - concept:layered-config-sources
  - concept:config-overlay
  - concept:reusable-source-parsers
  - flow:config-load
  - requirement:source-provenance-logging
  - requirement:cli-subcommands
  - requirement:config-file-discovery
  - decision:config-file-path-resolution
  - decision:prefix-table-binding
  - decision:configbind-codegen-no-reflect
  - system:configdir
  - system:configbind
acceptance:
  - same Bind key in file and env uses env value
  - same Bind key in env and CLI uses CLI value
  - unset Bind key keeps default and provenance marks default
  - keys under [webservice] only affect Bind(..., "webservice")
  - subcommand selection does not require TOML or env keys
  - overlay Place feeds provenance helper
  - --config-path selects the file over configdir search
  - --config-path unreadable returns error without falling back to configdir
```
