---
id: rule:check-codegen-pipeline
type: rule
title: Check Validation Codegen Pipeline
---
Generated bind validates first, then applies defaults; failures become validation errors without runtime tag parsing.

```yaml
order:
  - bind fields from request
  - run validateXxx on bound values
  - apply defaults for still-absent values
  - return value or error
default_source: default tag per rule:default-tag-semantics; check carries only failing rules
rationale:
  - check before default lets defaults sit outside valid ranges as sentinels
  - example: 'check:"min=1" default:"-1"' distinguishes undefined (becomes -1) from explicit invalid -1 (fails min)
  - default-first would validate the sentinel and reject legitimate absences
validate_presence:
  optional_absent: skip value constraints (min/max/minlen/format/enum/pattern) when field absent
  required_absent: required fails before default
  present_invalid: fail (e.g. explicit -1 with min=1) and never apply default for that field
default_presence:
  query_path_header: apply default only when key was absent and validation passed
  body_json: no presence without pointer; default limited or documented
  note: defaults may be outside check ranges by design (sentinel pattern)
generated_shape: |
  func bindCreateUserRequest(r *http.Request) (CreateUserRequest, error) {
      // bind fields...
      if err := validateCreateUserRequest(&out, presence); err != nil {
          return out, err
      }
      applyDefaultsCreateUserRequest(&out, presence)
      return out, nil
  }
errors:
  style: fixed English templates per rule
  map_to: httpbind.Validation / Field style problem details
  custom_messages: deferred past v1
  i18n: deferred
messages_examples:
  required: required
  min: "must be >= N"
  minlen: "length must be >= N"
  uuid: "must be a valid uuid"
  enum: "must be one of: a, b"
  date: "must be ISO date"
related:
  - concept:check-validation
  - concept:code-generation
  - api:bind
  - concept:error-helpers
  - rule:standard-error-mapping
  - decision:reflection-free
  - rule:check-required-semantics
  - rule:check-v1-rule-set
  - rule:default-tag-semantics
```
