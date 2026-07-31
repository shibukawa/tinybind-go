---
id: requirement:request-default-tag
type: requirement
title: Request Default Values from the default Tag
---
Request model fields declare defaults with `default:"value"`, generating the same assignment and OpenAPI metadata the check tag used to produce.

```yaml
priority: must
decision: decision:default-tag-form
semantics: rule:default-tag-semantics
behavior:
  - request field analysis reads the default tag alongside the check tag
  - a parsed default drives generated default assignment and the OpenAPI default keyword
  - a default tag makes the field presence-tracked, as 'check:"default="' did
  - 'default= inside check fails generation with a message pointing at the default tag'
  - check tag keeps rules that can reject a value, minus enum per requirement:request-enum-tag
  - config struct handling is untouched; it already reads the same tag form
acceptance:
  - 'Sort string `payload:"sort" default:"asc"` assigns out.Sort = "asc" when the key is absent'
  - 'Sort string `payload:"sort" default:"asc"` emits "default": "asc" in the OpenAPI schema'
  - 'Page int `payload:"page" default:"1"` assigns out.Page = 1 and documents default 1, instead of being ignored'
  - 'Page int `query:"page" check:"min=1" default:"-1"` keeps the sentinel: absent yields -1, explicit -1 fails min'
  - 'Sort string `check:"default=asc"` fails generation and names the default tag'
  - a value unparsable for the field type fails generation, as check default did
  - a default on a file, rest map, or nested composite field fails generation
  - generated code and OpenAPI output are byte-identical to the check-tag form for every migrated fixture
related:
  - decision:default-tag-form
  - rule:default-tag-semantics
  - requirement:request-enum-tag
  - rule:check-codegen-pipeline
  - rule:check-v1-rule-set
  - rule:openapi-validation-metadata
  - requirement:bind-check-validation
  - concept:check-validation
  - concept:request-binding
  - concept:openapi-generation
  - api:bind
```
