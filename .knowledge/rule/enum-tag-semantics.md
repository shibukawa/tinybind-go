---
id: rule:enum-tag-semantics
type: rule
title: Enum Tag Semantics
---
`enum:"a,b,c"` parses at generation time into a typed allowlist that rejects any present value outside the list.

```yaml
form: 'enum:"a,b,c"'
parser: codegen only; never read at runtime per decision:reflection-free
value_parsing:
  split: ","
  trim: spaces around each value
  empty_tag: generation error
  empty_value: generation error
  value_kinds: string, int, int64, bool, float64
  unparsable_value: generation error naming value and kind
  comma_in_value: not expressible; same limit the check tag had
type_applicability:
  allowed: scalar fields only
  rejected: file, payload rest map, nested struct, slice, map
is_validation: true
validation_effects:
  - field is presence-tracked
  - absent optional value skips the check
  - present value outside the list becomes a field error 'must be one of: a, b'
  - an enum-only field still emits validate code and still documents a 400 response
default_interaction:
  default_need_not_be_listed: true
  reason: preserves the sentinel pattern of rule:check-codegen-pipeline
  differs_from: rule:enum-value-validation, which requires config defaults to be listed
check_tag_interaction:
  'enum= inside check': generation error suggesting the comma-separated enum tag
openapi:
  keyword: enum
  mapping: rule:openapi-validation-metadata
related:
  - decision:enum-tag-form
  - requirement:request-enum-tag
  - rule:default-tag-semantics
  - rule:check-codegen-pipeline
  - rule:check-tag-syntax
  - rule:enum-value-validation
  - rule:openapi-validation-metadata
  - decision:struct-field-tags
  - decision:reflection-free
```
