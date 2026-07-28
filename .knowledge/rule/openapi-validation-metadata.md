---
id: rule:openapi-validation-metadata
type: rule
title: OpenAPI Validation Metadata from Struct Tags
---
Validation and documentation metadata for OpenAPI schemas is generated from struct tags, primarily concept:check-validation check tags.

```yaml
primary_source: concept:check-validation
default_source: default tag per rule:default-tag-semantics
enum_source: enum tag per rule:enum-tag-semantics
supported_metadata:
  - required
  - default
  - enum
  - minimum
  - maximum
  - minLength
  - maxLength
  - minItems
  - maxItems
  - pattern
  - format
  - deprecated
  - example
  - description
check_mapping: rule:check-v1-rule-set
source: Go struct tags
related:
  - concept:openapi-generation
  - concept:request-binding
  - concept:check-validation
  - rule:check-format-validators
  - rule:default-tag-semantics
  - rule:enum-tag-semantics
  - decision:check-tag-validation
  - decision:default-tag-form
  - decision:enum-tag-form
```
