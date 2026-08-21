---
id: requirement:scaffold-generation
type: requirement
title: Config Scaffold Generation
---
Each discovered Bind type-and-prefix registration contributes one configbind Definition; configbind public APIs render combined TOML and .env scaffolds from its scaffold fields.

```yaml
priority: must
intent: bootstrap shared config files from Bind structs only
delivery: one generated Definition registration per Bind plus public runtime aggregation
mechanism:
  - each package generator emits one Definition per discovered Bind type-and-prefix registration
  - generated package init registers each Definition once with configbind
  - api:config-scaffold-output merges scaffold fields from all registered definitions and returns or writes TOML and .env text
  - application owns any CLI command and destination file
inputs:
  - api:configbind-bind registrations only
  - decision:struct-field-tags for default, help, enum, dependon
  - data:cli-flag-def help text for comments
  - requirement:godoc-config-descriptions for struct and field doc text
  - decision:prefix-table-binding
  - decision:toml-shape-constraints
excluded_inputs:
  - api:configbind-subcommand types and fields
outputs:
  - combined TOML text with prefix tables, dotted nested keys, primitive arrays, and one example [[key]] block per struct slice
  - combined .env text using runtime environment naming and overrides
  - comments derived from help tags next to keys
  - struct doc comment lines above each [prefix] table header in TOML only
  - example values derived from default tags when present
  - >
    allowed-value notes from enum tags, one comment line under the help lines, per
    rule:enum-value-validation cli_and_scaffold
constraints:
  - codegen performs no runtime file write
  - codegen adds no application CLI command or subcommand
  - final application generation does not rescan framework or module dependency source
  - registry key includes the Go type and Bind prefix
  - diagnostic identity includes the Go package path and Bind type identity
  - table order does not depend on package init order; see rule:config-output-ordering
  - fields inside one table follow struct declaration order
  - dependon never removes a field from a scaffold; see rule:dependent-key-visibility
  - do not emit inline tables or quoted keys
  - nested structs become nested tables or dotted bare keys
  - struct slices become [[prefix.key]] blocks written after that prefix's own keys
  - .env output omits struct slices, which have no environment form
  - do not include subcommand options or positionals
  - opt CLI renames do not change TOML key names in the scaffold
related:
  - requirement:modular-package-generation
  - requirement:deterministic-config-output-order
  - requirement:dependent-field-visibility
  - requirement:duration-config-fields
  - rule:config-output-ordering
  - rule:dependent-key-visibility
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
  - scaffold TOML lists a table's keys in struct declaration order
  - a field with dependon still appears in both scaffolds
  - scaffold TOML quotes duration defaults such as "5s"
  - scaffold TOML lines include help as comments when help tag present
  - scaffold TOML shows the struct doc comment above its [prefix] header
  - scaffold env shows field help comments and no struct doc
  - default values appear as example values when default tag present
  - scaffold env uses runtime names including opt and env overrides and omits env:"-" fields
  - subcommand-only fields never appear in scaffolds
  - duplicate prefix-key or environment names produce an aggregation error
```
