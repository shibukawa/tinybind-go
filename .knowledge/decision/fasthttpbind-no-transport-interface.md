---
id: decision:fasthttpbind-no-transport-interface
type: decision
title: No Shared Transport Interface
---
Generate two concrete backends against their native types instead of defining a Request and Writer interface both transports implement.

```yaml
status: proposed 2026-08-08
rejected_shape: |
  type Request interface { Query(string) (string, bool); Header(string) string; ... }
  func Bind[T any](r Request) (T, error)
why_rejected:
  cost_erases_gain: an interface call per field access reintroduces the indirection fasthttp exists to remove, so the abstraction spends the whole budget it was built to save
  moves_the_author_shape: the framework-facilities principle admits a seam only when default output is byte-identical and the contract stays with the caller, and refuses changes that move what the application author writes; Bind(r Request) moves it for every existing httpbind handler
  no_honest_common_subset: RequestCtx lifetime under rule:fasthttpbind-requestctx-lifetime has no net/http counterpart, so either the interface leaks the stricter contract to net/http callers who do not need it, or it hides it from fasthttp callers who do
consequence_accepted:
  fact: the handler signature differs per backend, concept:net-http-handler against concept:fasthttp-handler
  mitigation: decision:transport-neutral-handler gives application code a form with no transport in it at all, which is the portable surface
  not_mitigated: a handler that names ResponseWriter and Request is transport-bound by declaration, and stays so
single_source_kept:
  shared: the field plan, the check rules, the OpenAPI schema, the wire names
  per_backend: only the emitted access expressions
related:
  - decision:fasthttpbind-runtime-package
  - decision:transport-neutral-handler
  - concept:fasthttp-handler
  - concept:net-http-handler
  - rule:fasthttpbind-requestctx-lifetime
```
