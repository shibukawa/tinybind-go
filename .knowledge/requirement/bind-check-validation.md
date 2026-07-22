---
id: requirement:bind-check-validation
type: requirement
title: Bind Executes Check Validation
---
api:bind must complete generated field-level concept:check-validation before returning success.

```yaml
priority: must
request_contract:
  source: Go request struct check tags
  execution: generated binder called by api:bind
  runtime_reflection: forbidden
behavior:
  - bind request fields before validation
  - evaluate every applicable check rule in rule:check-codegen-pipeline
  - aggregate field violations into one httpbind.Validation error
  - identify each violation by wire field name, binding location, and fixed rule message
  - return the validation error through api:bind
  - apply defaults only after successful validation
error_location:
  explicit_tags: use the declared query, payload, path, header, cookie, or method location
  default_input: use input because the same field can bind from query or body
handler_contract:
  - handle the api:bind error with api:write-error
  - do not repeat check-tag field validation with handwritten httpbind.Validation or httpbind.Field
  - retain handwritten validation for domain and cross-field rules outside check v1
acceptance:
  - 'Name string `payload:"name" check:"required"` rejects an absent or empty name during api:bind'
  - 'Email string `payload:"email" check:"required,email"` reports payload field errors without handler checks'
  - multiple invalid fields are returned together
  - valid bound values are returned normally
  - fields without check rules preserve binding behavior
  - generated code contains no runtime tag parsing
example_handler: |
  input, err := httpbind.Bind[CreateUserRequest](r)
  if err != nil {
      httpbind.WriteError(w, r, err)
      return
  }
related:
  - api:bind
  - concept:check-validation
  - rule:check-codegen-pipeline
  - concept:error-helpers
  - policy:problem-details
  - decision:check-tag-validation
  - decision:reflection-free
```
