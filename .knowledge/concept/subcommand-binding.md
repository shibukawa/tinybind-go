---
id: concept:subcommand-binding
type: concept
title: Subcommand Binding
---
CLI-only subcommands select at most one option struct; API returns *T or nil.

```yaml
model:
  - each SubCommand[T](name, help) is a named CLI branch
  - selected branch returns non-nil *T filled from CLI
  - unselected branches return nil
  - no TOML table or env namespace for subcommand fields
fields:
  - option fields: CLI flags with default/help tags
  - positional fields: arg required|optional|*
return_type: '*T'
nil_semantics:
  - nil means subcommand not invoked
  - non-nil means selected and parsed successfully
related:
  - api:configbind-subcommand
  - requirement:cli-subcommands
  - decision:struct-field-tags
  - concept:cli-option-codegen
  - flow:config-load
```
