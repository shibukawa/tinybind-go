---
id: requirement:scaffold-generation
type: requirement
title: Config Scaffold Generation
---
Each discovered Bind type-and-prefix registration contributes data:config-scaffold-fragment; configbind public APIs render combined TOML and .env scaffolds.

```yaml
priority: must
intent: bootstrap shared config files from Bind structs only
delivery: generated per-struct registration plus public runtime aggregation
mechanism:
  - each package generator emits one fragment per discovered Bind type-and-prefix registration
  - generated package init registers fragments with configbind
  - api:config-scaffold-output merges all registered fragments and returns or writes TOML and .env text
  - application owns any CLI command and destination file
inputs:
  - api:configbind-bind registrations only
  - decision:struct-field-tags for default, help, enum
  - data:cli-flag-def help text for comments
  - decision:prefix-table-binding
  - decision:toml-shape-constraints
excluded_inputs:
  - api:configbind-subcommand types and fields
outputs:
  - combined TOML text with prefix tables, dotted nested keys, and primitive arrays
  - combined .env text using runtime environment naming and overrides
  - comments derived from help tags next to keys
  - example values derived from default tags when present
  - optional allowed-value notes from enum tags
constraints:
  - codegen performs no runtime file write
  - codegen adds no application CLI command or subcommand
  - final application generation does not rescan framework or module dependency source
  - registration never replaces another struct fragment
  - fragment identity includes Go package path, Bind type identity, and prefix
  - output order does not depend on package init order
  - do not emit inline tables, arrays of tables, or quoted keys
  - nested structs become nested tables or dotted bare keys
  - do not include subcommand options or positionals
  - opt CLI renames do not change TOML key names in the scaffold
related:
  - requirement:modular-package-generation
  - data:config-scaffold-fragment
  - api:config-scaffold-output
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
  - framework and application Bind structs generated in separate invocations appear in one output
  - modular-monolith packages contribute without a whole-application source scan
  - same-named config types in different packages coexist
  - application-owned code can call public configbind output functions
  - scaffold TOML contains [prefix] tables for each Bind
  - scaffold TOML lines include help as comments when help tag present
  - default values appear as example values when default tag present
  - scaffold env uses runtime names including opt and env overrides and omits env:"-" fields
  - subcommand-only fields never appear in scaffolds
  - duplicate prefix-key or environment names produce an aggregation error
```
