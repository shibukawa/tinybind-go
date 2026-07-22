---
id: concept:error-helpers
type: concept
title: Error Helpers
---
Convenience constructors for HTTP errors, validation fields, and optional wrapped causes.

```yaml
status_helpers:
  - httpbind.BadRequest
  - httpbind.Unauthorized
  - httpbind.Forbidden
  - httpbind.NotFound
  - httpbind.Conflict
  - httpbind.PayloadTooLarge
  - httpbind.Internal
  - httpbind.Validation
openapi_discovery: rule:error-response-discovery
symbol_identity: rule:go-types-symbol-identity

problem_payload: data:problem
built_in_examples:
  - |
    return UserResponse{}, httpbind.BadRequest(
        Problem{Code: "invalid_email", Message: "email is invalid"},
    )
  - |
    return UserResponse{}, httpbind.NotFound(
        Problem{Code: "user_not_found", Message: "user not found"},
    )
  - |
    return UserResponse{}, httpbind.Conflict(
        Problem{Code: "duplicate_email", Message: "email already exists"},
    )
  - "return UserResponse{}, httpbind.Internal(err)"
validation_example: |
  return UserResponse{}, httpbind.Validation(
      httpbind.Field("email", "payload", "must be a valid email"),
      httpbind.Field("age", "payload", "must be greater than or equal to 18"),
  )
generated_validation: concept:check-validation
bind_validation: requirement:bind-check-validation
note: handwritten Validation/Field remains for domain and cross-field rules; field-level input checks move to check tags and api:bind
field_helper:
  name: httpbind.Field
  args:
    - field name
    - location (input|payload|query|path|header|cookie|method)
    - message
cause_wrapping: rule:error-cause-preservation
response_writer: api:write-error
related:
  - policy:problem-details
  - rule:standard-error-mapping
  - rule:error-response-discovery
  - data:problem
  - concept:check-validation
  - requirement:bind-check-validation
  - rule:check-codegen-pipeline
```
