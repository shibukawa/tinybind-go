---
id: decision:default-tag-form
type: decision
title: Default Value Tag Form
---
Request models and config structs both declare default values with a standalone `default:"value"` struct tag; `default=` inside a check tag is removed.

```yaml
status: accepted
problem:
  - two notations expressed one idea
  - request models used 'check:"default=asc"' via rule:check-tag-syntax
  - config structs used 'default:"8080"' via decision:struct-field-tags
  - a default tag on a request field was read by nobody and failed silently
chosen: standalone default tag on both sides
rationale:
  - a default is not a constraint; it never rejects a value
  - check tag keeps one job: rules that can fail
  - config form already carries the tag; aligning moves one side, not two
  - standalone tag avoids the CSV escaping limits of rule:check-tag-syntax
  - silent no-op on request fields becomes impossible
alternatives_rejected:
  - keep check default=: leaves the split the change exists to close
  - accept both with default tag winning: two documented spellings, no removal date
  - accept both plus deprecation diagnostic: needs a new warning channel for a v0 break not worth staging
migration:
  policy: breaking, no compatibility window
  version_context: v0.1.x, pre-1.0
  generator_behavior: 'default= inside check is a generation error naming the default tag'
  in_repo_call_sites: 4
  affected_files:
    - examples/demo/types.go
    - internal/checkfixture/types.go
    - docs/httpbind.md
    - docs/httpbind.ja.md
unchanged_by_this_decision:
  - default application timing in rule:check-codegen-pipeline
  - sentinel pattern where a default sits outside a check range
  - OpenAPI default emission in rule:openapi-validation-metadata
followed_by:
  decision:enum-tag-form: same alignment applied to enum, which was the open question left here
related:
  - requirement:request-default-tag
  - rule:default-tag-semantics
  - decision:enum-tag-form
  - decision:struct-field-tags
  - decision:check-tag-validation
  - decision:single-source-of-truth
  - concept:check-validation
  - concept:request-binding
```
