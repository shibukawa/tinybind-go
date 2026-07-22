---
id: api:bind
type: api
title: httpbind.Bind
---
Generic request binder that maps *http.Request into a typed request struct using generated code.

```yaml
signature: "func Bind[T any](r *http.Request) (T, error)"
example: "input, err := httpbind.Bind[CreateUserRequest](r)"
behavior:
  - bind query, payload, path, header, cookie, method per field tags
  - validate check tags then apply defaults per requirement:bind-check-validation
  - return typed value or error
  - no runtime reflection
uses:
  - concept:request-binding
  - concept:code-generation
  - concept:check-validation
  - requirement:bind-check-validation
  - rule:default-input-tag
  - rule:check-codegen-pipeline
discovery: rule:request-model-discovery
error_path: api:write-error
related:
  - system:tinybind
  - concept:net-http-handler
  - concept:handler-discovery
  - concept:error-helpers
```
