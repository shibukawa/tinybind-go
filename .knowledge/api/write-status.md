---
id: api:write-status
type: api
title: httpbind.WriteStatus
---
Typed response writer with explicit HTTP success status; preferred over wrapping Body in a Response struct for TinyGo and codegen simplicity.

```yaml
status: implemented
signature: "func WriteStatus[T any](w http.ResponseWriter, r *http.Request, status int, value T) error"
examples:
  - "httpbind.WriteStatus[CreateUserResponse](w, r, http.StatusCreated, out)"
  - "httpbind.WriteStatus[struct{}](w, r, http.StatusNoContent, struct{}{})"
behavior:
  - write status then serialize value as JSON (or empty body policy for 204)
  - no runtime field reflection on T
  - serialize through jsonbind.EncodeJSON, which reads the encoder registry, not the writer registry api:write reads
registry: |
  the encoder one, which is not the path api:write takes. This line read "same
  codec path as api:write generated writers where possible" until 2026-08-09,
  and the generator followed the wording rather than the code: a write-status
  call site was mapped to UsageWrite alone, so the encoder body was emitted and
  never registered. rule:call-operation-usage now holds the mapping.
status_defaults:
  api:write: always 200 OK today
  WriteStatus: caller-supplied status
common_statuses:
  - 200 OK
  - 201 Created
  - 202 Accepted
  - 204 No Content
  - any other success status int
no_content:
  status_204: body may be empty or ignored; document generator behavior for empty T
rejected_alternative: |
  Write(w, r, Response{Status: n, Body: v}) deferred; WriteStatus keeps one type param and simpler emit
discovery: rule:response-model-discovery
openapi: rule:openapi-success-status
related:
  - api:write
  - concept:response-binding
  - concept:code-generation
  - rule:call-operation-usage
  - rule:openapi-success-response
  - rule:openapi-success-status
  - system:tinybind
```
