---
id: decision:enum-tag-form
type: decision
title: Enum Tag Form
---
Request models and config structs both declare allowed values with a standalone `enum:"a,b,c"` struct tag; `enum=` inside a check tag is removed.

```yaml
status: accepted
follows: decision:default-tag-form
problem:
  - request models used 'check:"enum=asc|desc"' via rule:check-tag-syntax
  - config structs used 'enum:"asc,desc"' via decision:struct-field-tags
  - same allowlist idea, two spellings and two separators
chosen: standalone enum tag with comma-separated values on both sides
rationale:
  - one spelling for one idea, matching the default tag alignment
  - config form already carries the tag; aligning moves one side, not two
  - pipe separator existed only to dodge the CSV split of rule:check-tag-syntax
  - standalone tag has no rule-separator to dodge
separator_change:
  from: "|"
  to: ","
  regression: none
  reason: a comma inside a check enum value was already impossible, since commas split check rules
differs_from_default:
  - an enum can reject a value, so it is validation
  - a field carrying only an enum tag still generates validate code and still documents a 400 response
  - see rule:enum-tag-semantics
migration:
  policy: breaking, no compatibility window
  version_context: v0.1.x, pre-1.0
  generator_behavior: 'enum= inside check is a generation error suggesting the comma-separated enum tag'
  in_repo_call_sites: 2
  affected_files:
    - internal/checkfixture/types.go
    - docs/httpbind.md
    - docs/httpbind.ja.md
  verification: generated bind and OpenAPI output byte-identical before and after, stamp line excluded
not_adopted_from_config:
  - 'rule:enum-value-validation requires a config default to be a listed enum value; request models keep the sentinel freedom of rule:check-codegen-pipeline instead'
related:
  - decision:default-tag-form
  - requirement:request-enum-tag
  - rule:enum-tag-semantics
  - rule:enum-value-validation
  - decision:struct-field-tags
  - decision:check-tag-validation
  - decision:single-source-of-truth
  - concept:check-validation
  - concept:request-binding
```
