---
id: requirement:configbind-product-goals
type: requirement
title: configbind Product Goals
---
Core product goals for the configbind configuration package.

```yaml
goals:
  - register any number of config structs via Bind[T](prefix)
  - map shared settings onto registered structs under prefix tables
  - multi-source load for Bind only: TOML file, env, CLI
  - single TOML file path via --config-path or configdir user-then-system search
  - never merge user and system config file contents
  - later source wins on the same Bind key
  - code-generate CLI option parsers and struct apply without reflection
  - reusable source parsers feed a provenance-aware overlay then generated apply
  - provenance log helper returns []{Key, Value, Place} for Bind keys
  - secret tag and sensitive key names redact or omit log Values
  - TOML-only config files in v1 with restricted shape
  - TinyGo is a first-class runtime target
  - field types limited to web-server options plus primitive arrays and nested structs
  - defaults, help, opt, enum, and secret tags live on struct field tags
  - CLI flags default to --prefix-key; opt overrides and suppresses default name
  - generator extracts CLI flag defs from structs
  - generated per-struct fragments compose into TOML and .env scaffold output
  - help tags feed CLI usage and TOML scaffold comments
  - SubCommand[T](name, help) returns *T or nil
  - subcommands are CLI-only; no TOML or env for subcommand fields
  - subcommand positionals via arg required|optional|*
maps_to:
  - system:configbind
  - vision:configbind
  - requirement:struct-registration
  - requirement:layered-config-load
  - requirement:cli-option-codegen
  - requirement:source-provenance-logging
  - requirement:configbind-tinygo
  - requirement:struct-field-metadata
  - requirement:scaffold-generation
  - requirement:cli-subcommands
  - requirement:config-file-discovery
  - decision:config-file-path-resolution
  - system:configdir
  - decision:toml-config-format
  - decision:toml-shape-constraints
  - decision:prefix-table-binding
  - decision:configbind-supported-types
  - decision:struct-field-tags
  - decision:cli-flag-naming
  - decision:configbind-runtime-architecture
  - decision:configbind-codegen-no-reflect
  - data:cli-flag-def
  - rule:source-precedence
  - rule:toml-shape-validation
  - rule:enum-value-validation
  - rule:secret-redaction
  - concept:config-struct-mapping
  - concept:subcommand-binding
  - concept:scaffold-templates
  - concept:provenance-log-helper
  - concept:config-overlay
  - concept:reusable-source-parsers
  - data:provenance-event
  - data:overlay-entry
  - api:configbind-bind
  - api:configbind-subcommand
acceptance:
  - app can Bind N independent config structs with distinct prefixes
  - Bind[WebServiceConfig]("webservice") maps TOML table [webservice]
  - --config-path overrides configdir user/system file discovery
  - --config-path unreadable or missing fails load without fallback
  - discovery API takes vendor name, tool name, and file name
  - user config file preferred over system; only one file is read
  - nested struct fields map via nested standard tables or dotted bare keys
  - primitive array fields map via TOML arrays of scalars
  - inline tables, arrays of tables, and quoted keys are rejected
  - default tag values apply when no external source sets the Bind key
  - help tags drive CLI usage and Bind TOML scaffold comments
  - default CLI flag for [webserver] port is --webserver-port
  - opt:"port,p" yields --port and -p without --webserver-port
  - enum tags reject values outside the allowlist from any source
  - public configbind APIs merge framework and application scaffold fragments
  - scaffolds cover Bind fields only, never SubCommand fields
  - SubCommand returns *T when selected and nil when not
  - arg required|optional|* parse positionals for subcommands
  - SubCommand fields ignore TOML and env
  - final Bind field values equal merge of defaults, TOML, env, CLI
  - generated CLI parses flags matching Bind struct fields under prefix
  - provenance helper returns []{Key, Value, Place} with redacted Values
  - secret:"hide" omits keys; secret:"mask" uses asterisks with length jitter
  - sensitive key names auto-mask; others default to show
  - package and generated code build with TinyGo
  - runtime does not use reflection for struct field bind
  - source parsers remain reusable map producers not tied to app structs
  - scalar field types are bool, int, string, duration, datetime, url only
```
