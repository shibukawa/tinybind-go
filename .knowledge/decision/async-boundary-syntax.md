---
id: decision:async-boundary-syntax
type: decision
title: Await Fallback Recover Syntax
---
Use one three-state boundary syntax for pending, successful, and failed asynchronous component rendering, with the await clause binding its own results.

```yaml
source:
  - concept:html-render-runtime-extensions
  - user syntax discussion 2026-07-22
  - user syntax decision 2026-07-25
  - user binding decision 2026-07-26
review_gate: approved 2026-07-25; binding form approved 2026-07-26
shape: |
  {await user = LoadUser(id), posts = LoadPosts(id)}
    primary subtree
  {fallback}
    pending subtree
  {recover err}
    failure subtree
  {/await}
semantics:
  await: binds one or more requirement:async-external-functions calls; the primary subtree reads the bound names
  fallback: emitted and flushed while dependencies are pending; required, because a boundary always commits something first
  recover: replaces fallback when a dependency returns error, panics, or times out; optional
  error: typed data:async-render-error bound by the recover clause; the raw Go error is unavailable
binding_form:
  chosen: explicit binding at the wait site, like the loop variable of a for clause
  rejected: propagating a pending effect through the component call graph until an await clause encloses it
  reason:
    - the dependency of a boundary is readable at the boundary instead of inferred from a whole subtree
    - typing needs no effect system; a bound name is an ordinary typed identifier in the primary scope
    - an async call outside an await clause is a local error with a precise position
  consequence: a component cannot make its caller asynchronous; every wait site is written where it is awaited
scoping:
  primary: outer scope plus the bound names
  fallback: outer scope only; the bound names do not exist yet
  recover: outer scope plus the error name, and never the bound names
concurrency: bindings of one await clause start together and settle together; the first failure decides the boundary
compiler:
  - fallback and recover clauses must be synchronously renderable
  - a nested await clause inside a primary subtree opens its own boundary
  - an await clause inside a for body opens one boundary per iteration
  - a requirement:html-slot-syntax slot may not appear in any await clause, because fallback and primary would both render it
  - expected request cancellation and stale partial-update completion bypass recover
  - each boundary yields at most one data:async-boundary-content item after the initial document write
omitted_recover:
  behavior: the committed fallback stays in place and no completion is emitted for that boundary
  reporting: the normalized failure goes to the render error hook, so it stays observable server-side
naming:
  benefit: await marks the wait site; async stays the external declaration modifier; fallback preserves the pending term; recover describes error UI replacement
  rejected: async clause keyword; it collided with the async external modifier and read as a declaration rather than a wait site
  caveat: unlike Go recover, this clause handles returned errors and timeouts as well as normalized panics
```
