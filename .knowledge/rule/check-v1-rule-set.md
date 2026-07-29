---
id: rule:check-v1-rule-set
type: rule
title: Check Validation v1 Rule Set
---
v1 check rules cover presence, numeric bounds, lengths, patterns, and ISO format shortcuts.

```yaml
presence:
  - required
own_tag_not_check_rule:
  default: rule:default-tag-semantics
  enum: rule:enum-tag-semantics
default_timing: after validate; see rule:check-codegen-pipeline
default_may_be_out_of_range: true
numeric_inclusive:
  - min
  - max
length:
  - minlen
  - maxlen
  - len
pattern:
  - pattern
format_shortcuts:
  - uuid
  - email
  - date
  - time
  - datetime
type_applicability:
  min_max: numeric types only
  minlen_maxlen_len: string and slice
  required: see rule:check-required-semantics
  format_shortcuts: string primarily; skip empty unless required
deferred_optional:
  - gt
  - gte
  - lt
  - lte
  - uri
  - url
  - format= generic sugar
excluded_v1:
  - eq / ne
  - cross-field
  - dive / element validation beyond outer length
  - unique
  - alpha / alphanum / contains family
  - file size / MIME (separate File rules later)
openapi_map:
  required: required / parameter required
  min: minimum
  max: maximum
  minlen: minLength or minItems
  maxlen: maxLength or maxItems
  len: minLength+maxLength or minItems+maxItems
  pattern: pattern
  email: format email
  uuid: format uuid
  date: format date
  time: format time
  datetime: format date-time
  default: default keyword, sourced from the default tag not from check
  enum: enum keyword, sourced from the enum tag not from check
related:
  - concept:check-validation
  - rule:check-tag-syntax
  - rule:default-tag-semantics
  - rule:enum-tag-semantics
  - rule:check-required-semantics
  - rule:check-format-validators
  - rule:openapi-validation-metadata
```
