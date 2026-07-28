---
id: rule:render-external-query-semantics
type: rule
title: Render External Query Semantics
---
Treat external functions called during HTML rendering as repeatable data queries rather than state-changing commands.

```yaml
source: requirement:generated-route-registration
rationale:
  - full render, partial navigation, cache miss, retry, and direct boundary update may execute the same component more than once
  - requirement:async-external-functions scheduling may overlap or cancel calls
requirements:
  - no externally visible mutation or exactly-once side effect
  - deterministic output for cache-enabled components given declared inputs and dependency version
  - honor context cancellation for I/O where practical
  - return typed result and error; do not write http.ResponseWriter directly
  - authorization is enforced from trusted request context, never client continuation alone
allowed:
  - database reads, service queries, derived calculations, and idempotent fetches
forbidden:
  - payment, email send, record mutation, counter increment, or other command triggered merely by render
actions:
  status: future explicit action or command API with CSRF, method, validation, idempotency, and redirect or delta semantics
cache:
  - undeclared request, user, locale, or authorization dependencies make component output ineligible for reuse
```
