---
id: rule:request-model-discovery
type: rule
title: Request Model Discovery
---
Request models are discovered from the generic type argument of httpbind.Bind[T](r).

```yaml
detection_call: "httpbind.Bind[T](r)"
example: "input, err := httpbind.Bind[CreateUserRequest](r)"
model_source: generic type argument T
symbol_identity: rule:go-types-symbol-identity
must_be: github.com/shibukawa/tinybind-go.Bind
reject: same-named Bind from other packages
alias_ok: true
scope:
  reads: every call site in the package being analyzed
  consults_no_registration: a handler needs no discovered registration for its request model to be found, which is why requirement:action-request-binding is a package list rather than a registration-site list
  skips: rule:generated-source-not-discovered
related:
  - api:bind
  - concept:request-binding
  - concept:handler-discovery
  - concept:openapi-generation
  - rule:go-types-symbol-identity
```

