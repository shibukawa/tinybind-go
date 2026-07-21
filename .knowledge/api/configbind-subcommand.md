---
id: api:configbind-subcommand
type: api
title: configbind.SubCommand
---
Generic SubCommand registers a CLI-only subcommand option struct and returns *T when selected, nil otherwise.

```yaml
signature_sketch: 'func SubCommand[T any](name string, help string) *T'
return_type: '*T'
nil_semantics:
  - non-nil *T when CLI selected this subcommand and parse succeeded
  - nil when not selected
behavior:
  - registers subcommand name and help for CLI dispatch
  - parses CLI flags and positionals on T only
  - does not participate in TOML or env layers
  - parses positional fields tagged arg required|optional|*
example:
  go: |
    migrate := configbind.SubCommand[MigrateOpt]("migrate", "run migrations")
    // app migrate ./db --dry-run  -> migrate != nil
    // other subcommand or none    -> migrate == nil
depends_on:
  - requirement:cli-subcommands
  - decision:struct-field-tags
  - decision:configbind-supported-types
related:
  - api:configbind-bind
  - concept:subcommand-binding
  - flow:config-load
  - requirement:cli-option-codegen
  - system:configbind
```
