---
id: rule:fasthttpbind-requestctx-lifetime
type: rule
title: RequestCtx Lifetime And Copy-On-Bind
---
Generated fasthttp binders must copy every value out of pooled request memory, and no bound value may alias the RequestCtx.

```yaml
status: proposed 2026-08-08
fact: RequestCtx and its byte slices are pooled and reused once the handler returns, so any surviving reference reads another request
copy_required_from:
  - QueryArgs and PostArgs values
  - header and cookie values
  - path parameter values supplied by the router
  - PostBody, including the JSON document a binder parses
  - multipart part contents bound into a File field
guaranteed_by_go: []byte to string conversion copies, so a binder producing string fields is already safe by construction
forbidden:
  - unsafe string aliasing of a pooled buffer, however tempting the allocation saving
  - a File value whose Content slice points into the multipart buffer
  - retaining the ctx, or a context derived from it, past handler return
  - handing the ctx to a goroutine that outlives the handler
temptation_named:
  what: zero-copy binding is the obvious next optimisation once binding is the visible cost
  why_refused: it converts a correctness property the type system currently enforces into a convention, and the failure is a wrong value under load rather than a panic
  revisit_only_if: a lifetime-scoped view type makes escape a compile error
enforcement:
  emitter: generated binders build owned values only; no emitted expression yields a slice of ctx memory
  test: a suite that binds, returns, serves an unrelated request from the same pooled ctx, then asserts the earlier bound value is unchanged
  concurrency: run that suite under -race with pooled contexts reused across goroutines
tinygo_note: TinyGo sync.Pool never evicts and its single lock is contended per system:tinygodriver-fasthttp, so pooled entries live longer there and a stale alias survives longer before it is noticed
related:
  - api:fasthttpbind-bind
  - decision:fasthttpbind-runtime-package
  - system:tinygodriver-fasthttp
  - concept:request-binding
```
