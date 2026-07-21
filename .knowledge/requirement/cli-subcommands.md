---
id: requirement:cli-subcommands
type: requirement
title: CLI Subcommands
---
Subcommands are CLI-only; SubCommand[T] returns *T when selected and nil otherwise.

```yaml
priority: must
api: api:configbind-subcommand
return_type: '*T'
behavior:
  - declare subcommands with SubCommand[T](name, help)
  - if CLI selects that subcommand, return non-nil *T populated from CLI flags and args
  - if not selected, return nil
  - positional args use arg tags on T
  - option fields on T are CLI flags only
sources:
  - CLI flags and positionals only
  - not TOML
  - not env
arg_roles:
  - 'arg:"required"': must be present
  - 'arg:"optional"': may be absent
  - 'arg:"*"': remaining args as multi-value
option_fields:
  - default and help tags via decision:struct-field-tags
  - same supported types as decision:configbind-supported-types
  - defaults apply only within the selected subcommand CLI parse
acceptance:
  - return type is *T
  - only the selected subcommand value is non-nil
  - missing required arg fails parse with usage help
  - optional arg absence leaves zero or default
  - rest arg captures remaining tokens
  - subcommand help string appears in top-level usage
  - subcommand fields never load from TOML or env
related:
  - api:configbind-subcommand
  - concept:subcommand-binding
  - decision:struct-field-tags
  - requirement:cli-option-codegen
  - requirement:layered-config-load
  - flow:config-load
  - system:configbind
```
