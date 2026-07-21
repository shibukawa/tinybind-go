---
id: requirement:scaffold-generation
type: requirement
title: Config Scaffold Generation
---
Generated app CLI subcommands print Bind-based TOML and .env scaffold plain text to stdout.

```yaml
priority: must
intent: bootstrap shared config files from Bind structs only
delivery: generated code subcommands, not a separate generator CLI
mechanism:
  - codegen embeds plain text scaffold bodies from struct metadata
  - scaffold subcommands write that text to stdout
subcommands:
  - id: scaffold-toml
    description: print sample TOML for Bind prefixes
  - id: scaffold-env
    description: print sample .env for Bind prefixes
inputs:
  - api:configbind-bind registrations only
  - decision:struct-field-tags for default, help, enum
  - data:cli-flag-def help text for comments
  - decision:prefix-table-binding
  - decision:toml-shape-constraints
excluded_inputs:
  - api:configbind-subcommand types and fields
outputs:
  - stdout TOML text with nested tables and primitive arrays
  - stdout .env text with prefixed keys
  - comments derived from help tags next to keys
  - example values derived from default tags when present
  - optional allowed-value notes from enum tags
constraints:
  - plain text only; print to stdout
  - no file write required by the scaffold subcommand itself
  - do not emit inline tables, arrays of tables, or quoted keys
  - nested structs become nested tables or dotted bare keys
  - do not include subcommand options or positionals
  - opt CLI renames do not change TOML key names in the scaffold
related:
  - flow:configbind-codegen
  - concept:scaffold-templates
  - requirement:struct-field-metadata
  - requirement:cli-option-codegen
  - requirement:cli-subcommands
  - decision:struct-field-tags
  - decision:cli-flag-naming
  - data:cli-flag-def
  - system:configbind
acceptance:
  - generated binary exposes scaffold-toml and scaffold-env style subcommands
  - invoking them prints template text to stdout
  - scaffold TOML contains [prefix] tables for each Bind
  - scaffold TOML lines include help as comments when help tag present
  - default values appear as example values when default tag present
  - scaffold env uses the same prefix key namespace as runtime env load
  - subcommand-only fields never appear in scaffolds
```
