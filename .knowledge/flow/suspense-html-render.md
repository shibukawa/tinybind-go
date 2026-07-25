---
id: flow:suspense-html-render
type: flow
title: Await Boundary Render Flow
---
Progressive request flow for asynchronous HTML boundaries.

```yaml
flow:
  trigger: caller starts ranging the decision:async-component-signature sequence of a requirement:suspense-html-streaming component
  steps:
    - id: schedule
      action: start required requirement:async-external-functions work under request context
    - id: fallback
      action: allocate boundary ID and write fallback into the initial response stream through the io.Writer argument
    - id: flush
      action: caller flushes initial bytes through the active encoding when supported
    - id: resolve
      action: receive typed completion through the response coordinator
    - id: render-outcome
      action: render primary on success or decision:async-boundary-syntax recover subtree with data:async-render-error on failure
    - id: yield
      action: yield data:async-boundary-content holding the boundary ID and rendered fragment
    - id: append-update
      action: caller appends identified template content and fixed replacement instruction, then flushes
    - id: finish
      action: sequence ends when all boundaries complete, the consumer stops, or request context cancels
  failure:
    before_commit: yield zero content with the render error and end the sequence
    async_error: yield checked recover content for the same boundary ID
    recover_render_error: preserve fallback and apply outer or server policy
    cancellation: yield nothing and end the sequence
```
