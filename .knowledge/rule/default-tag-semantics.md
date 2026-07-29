---
id: rule:default-tag-semantics
type: rule
title: Default Tag Semantics
---
`default:"value"` parses at generation time into a typed literal applied to an absent field after validation.

```yaml
form: 'default:"value"'
parser: codegen only; never read at runtime per decision:reflection-free
value_parsing:
  same_as: former check default= literal conversion
  kinds: string, int, int64, bool, float64
  unparsable_value: generation error naming field and kind
absent_vs_empty:
  missing_tag: no default
  'default:""': explicit empty-string default on string fields; honored and documented
  requirement: tag lookup must distinguish the two, not read a bare tag value
whitespace:
  trimmed: false
  reason: no separator to trim around, so surrounding spaces are part of the value
  contrast: rule:enum-tag-semantics trims, because comma-separated values need it
type_applicability:
  allowed: scalar fields only
  rejected: file, payload rest map, nested struct, slice, map
  reason: matches the v1 limit already applied to check rules
timing:
  order: bind, validate, apply defaults
  authority: rule:check-codegen-pipeline
  applies_when: field was absent and validation passed
  never_applies_when: value was present and rejected
presence_tracking:
  effect: a default tag makes the field presence-tracked
  scope_unchanged: body JSON presence limits carry over from rule:check-codegen-pipeline
sentinel_pattern:
  supported: default may sit outside a check range
  example: 'check:"min=1" default:"-1"'
check_tag_interaction:
  'default= inside check': generation error naming the default tag
  check_keeps: rules that can reject a value, minus enum which is its own tag per rule:enum-tag-semantics
openapi:
  keyword: default
  mapping: rule:openapi-validation-metadata
related:
  - decision:default-tag-form
  - requirement:request-default-tag
  - rule:check-codegen-pipeline
  - rule:check-tag-syntax
  - rule:check-v1-rule-set
  - rule:enum-tag-semantics
  - rule:openapi-validation-metadata
  - decision:struct-field-tags
  - decision:reflection-free
```
