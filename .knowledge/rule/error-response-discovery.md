---
id: rule:error-response-discovery
type: rule
title: Error Response Discovery
---
Recognized tinybind error constructors feed generated OpenAPI error responses.

```yaml
recognized_constructors:
  - httpbind.BadRequest
  - httpbind.Unauthorized
  - httpbind.Forbidden
  - httpbind.NotFound
  - httpbind.Conflict
  - httpbind.PayloadTooLarge
  - httpbind.Internal
  - httpbind.Validation
symbol_identity: rule:go-types-symbol-identity
must_be_package: github.com/shibukawa/tinybind-go/httpbind
alias_ok: true
note: import alias must still resolve; name-only match of BadRequest is insufficient
purpose: generate OpenAPI error responses
status_mapping: rule:openapi-error-statuses
media_type: application/problem+json
related:
  - concept:error-helpers
  - policy:problem-details
  - concept:openapi-generation
  - concept:handler-discovery
  - rule:openapi-error-statuses
  - rule:go-types-symbol-identity
```

