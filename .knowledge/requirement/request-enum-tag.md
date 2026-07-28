---
id: requirement:request-enum-tag
type: requirement
title: Request Enums from the enum Tag
---
Request model fields declare allowed values with `enum:"a,b,c"`, generating the same validation and OpenAPI metadata the check tag used to produce.

```yaml
priority: must
decision: decision:enum-tag-form
semantics: rule:enum-tag-semantics
behavior:
  - request field analysis reads the enum tag alongside the check and default tags
  - a parsed enum drives generated allowlist validation and the OpenAPI enum keyword
  - an enum makes the field presence-tracked and counts as validation
  - 'enum= inside check fails generation with a message suggesting enum:"a,b"'
  - check tag keeps min/max, lengths, pattern, required, and format shortcuts
  - config struct handling is untouched; it already reads the same tag form
acceptance:
  - 'Sort string `query:"sort" enum:"asc,desc"` rejects sort=other with "must be one of: asc, desc"'
  - 'Sort string `query:"sort" enum:"asc,desc"` emits "enum": ["asc","desc"] in the OpenAPI schema'
  - a field with only an enum tag still documents a 400 validation response
  - 'Sort string `enum:"asc,desc" default:"asc"` fills asc when absent and validates when present'
  - 'Sort string `check:"enum=asc|desc"` fails generation and suggests enum:"asc,desc"'
  - an empty enum tag, an empty value, or a value unparsable for the field type fails generation
  - an enum on a file, rest map, or nested composite field fails generation
  - generated code and OpenAPI output are byte-identical to the check-tag form for every migrated fixture
related:
  - decision:enum-tag-form
  - rule:enum-tag-semantics
  - requirement:request-default-tag
  - rule:check-codegen-pipeline
  - rule:check-v1-rule-set
  - rule:openapi-validation-metadata
  - requirement:bind-check-validation
  - concept:check-validation
  - concept:request-binding
  - concept:openapi-generation
  - api:bind
```
