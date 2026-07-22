---
id: system:configbind
type: system
title: configbind Package
---
Go package and generator that merge reusable source maps into an overlay and apply them via generated reflection-free code.

```yaml
package_name: configbind
role: web-server oriented configuration option loader
runtime_style: reusable source parsers plus overlay merge plus generated apply/CLI
architecture: decision:configbind-runtime-architecture
codegen: decision:configbind-codegen-no-reflect
config_file_format: TOML
tinygo: requirement:configbind-tinygo
components:
  reusable_parsers: concept:reusable-source-parsers
  overlay: concept:config-overlay
  generated_bind: concept:config-struct-mapping
  generated_cli: concept:cli-option-codegen
primary_inputs:
  - developer-defined Go structs via api:configbind-bind
  - optional CLI-only subcommand structs via api:configbind-subcommand
  - prefix string per Bind call (TOML table name)
  - field tags default, help, opt, enum, secret, arg via decision:struct-field-tags
  - CLI flag naming via decision:cli-flag-naming
  - TOML config file path for Bind fields only
  - vendor name, tool name, and file name for configdir discovery via API
  - optional --config-path process flag (error if unreadable)
  - process environment for Bind fields only
  - CLI args including subcommand name
config_path: decision:config-file-path-resolution
config_dirs: system:configdir
outputs:
  - populated Bind structs via generated apply
  - *T or nil from each SubCommand
  - generated CLI option and subcommand parser code
  - public TOML and .env scaffold output from registered data:config-scaffold-fragment values
  - provenance log records []{Key, Value, Place} for Bind keys
load_order_bind:
  - defaults into overlay
  - TOML map into overlay
  - env map into overlay
  - CLI map into overlay
  - generated apply onto structs
subcommand_sources:
  - CLI flags and positionals only
precedence: rule:source-precedence
namespace: decision:prefix-table-binding
field_types: decision:configbind-supported-types
toml_shape: decision:toml-shape-constraints
field_tags: decision:struct-field-tags
public_api:
  - api:configbind-bind
  - api:configbind-subcommand
related:
  - vision:configbind
  - requirement:configbind-product-goals
  - requirement:struct-registration
  - requirement:layered-config-load
  - requirement:cli-option-codegen
  - requirement:source-provenance-logging
  - requirement:configbind-tinygo
  - requirement:struct-field-metadata
  - requirement:scaffold-generation
  - requirement:cli-subcommands
  - flow:config-load
  - flow:configbind-codegen
  - decision:toml-config-format
  - decision:toml-shape-constraints
  - decision:prefix-table-binding
  - decision:configbind-supported-types
  - decision:struct-field-tags
  - decision:configbind-runtime-architecture
  - decision:configbind-codegen-no-reflect
  - rule:toml-shape-validation
  - concept:scaffold-templates
  - concept:subcommand-binding
  - concept:reusable-source-parsers
  - concept:config-overlay
  - data:overlay-entry
```
