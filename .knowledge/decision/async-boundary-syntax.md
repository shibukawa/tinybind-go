---
id: decision:async-boundary-syntax
type: decision
title: Await Fallback Recover Syntax
---
Use one three-state boundary syntax for pending, successful, and failed asynchronous component rendering.

```yaml
source:
  - concept:html-render-runtime-extensions
  - user syntax discussion 2026-07-22
  - user syntax decision 2026-07-25
review_gate: approved 2026-07-25
shape: await { primary subtree } fallback { pending subtree } recover(error) { failure subtree }
semantics:
  await: primary subtree may consume requirement:async-external-functions values
  fallback: emitted and flushed while dependencies are pending
  recover: replaces fallback when a dependency returns error, panics, or times out
  error: typed data:async-render-error; raw Go error is unavailable
compiler:
  - await clause introduces a boundary that consumes propagated pending effects
  - fallback and recover clauses must be synchronously renderable
  - recover cannot reference unavailable successful values
  - nested failure is handled by the nearest enclosing matching recover clause
  - expected request cancellation and stale partial-update completion bypass recover
  - enclosing component takes the async render signature in requirement:template-code-generation
  - each boundary yields at most one data:async-boundary-content item after the initial document write
naming:
  benefit: await marks the wait site; async stays the external declaration modifier; fallback preserves the pending term; recover describes error UI replacement
  rejected: async clause keyword; it collided with the async external modifier and read as a declaration rather than a wait site
  caveat: unlike Go recover, this clause handles returned errors and timeouts as well as normalized panics
  alternative: error(error) clause if Go panic association proves misleading
optional_recover:
  proposal: allow omission only when a configured safe default preserves fallback and logs the failure
```
