---
id: rule:check-tag-syntax
type: rule
title: Check Tag DSL Syntax
---
check tag is a compact CSV-like DSL of rule tokens; pattern is trailing-only in v1.

```yaml
form: 'check:"rule,rule=value,..."'
token_kinds:
  - bare: required, email, uuid, date, time, datetime
  - key_value: min, max, minlen, maxlen, len, pattern
not_a_check_rule:
  default: standalone tag per rule:default-tag-semantics; 'default=' inside check is a generation error
  enum: standalone tag per rule:enum-tag-semantics; 'enum=' inside check is a generation error
separators:
  rules: ","
pattern_policy:
  v1: pattern= must be last token in the tag
  reason: commas inside regex break CSV split
  alternatives_deferred:
    - semicolon rule separators
    - quoted pattern values
pattern_example: 'check:"required,pattern=^[A-Z]{3}$"'
not_compatible_with: go-playground/validator full dialect
parser: codegen only; never interpret tags at runtime
related:
  - concept:check-validation
  - rule:check-v1-rule-set
  - rule:default-tag-semantics
  - rule:enum-tag-semantics
  - decision:check-tag-validation
  - decision:default-tag-form
  - decision:enum-tag-form
  - decision:reflection-free
```
