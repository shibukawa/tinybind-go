---
id: rule:enum-value-validation
type: rule
title: Enum Value Validation
---
Fields with enum tags accept only the listed values from any source that sets the field.

```yaml
tag: 'enum:"a,b,c"'
scope: config structs; request models share the tag but not the default-must-be-listed rule, per rule:enum-tag-semantics
parse:
  - split on comma
  - trim spaces around each token, as request models do per rule:enum-tag-semantics
  - empty tokens rejected at generation or load time
enforcement:
  - after value is chosen from default, TOML, env, or CLI
  - value not in allowlist is a load/parse error
  - applies to Bind fields and SubCommand CLI fields when tagged
  - for []string with enum TBD_policy: each element must match or whole field must match one value
cli_and_scaffold:
  - usage/help may list allowed values
  - scaffold comments may list allowed values
related:
  - decision:struct-field-tags
  - decision:enum-tag-form
  - rule:enum-tag-semantics
  - requirement:struct-field-metadata
  - flow:config-load
  - requirement:cli-option-codegen
  - system:configbind
```
