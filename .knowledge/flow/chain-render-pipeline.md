---
id: flow:chain-render-pipeline
type: flow
title: Chain Render Pipeline Flow
---
Ordered execution of one requirement:chain-render-pipeline from chain assembly to the last streamed chunk.

```yaml
flow:
  trigger: a generated handler resolves a layout chain for an origin component
  steps:
    - id: assemble
      action: list the origin and its ordered layout ancestors from data:html-render-route-plan or a runtime override
    - id: classify
      action: mark the chain async when any member is async_boundary in data:component-render-capabilities
    - id: sync-path
      action: on a sync chain run the existing writer composition and return
    - id: schedule
      action: start requirement:async-external-functions work for every member under the request context
    - id: initial-pass
      action: write outermost frame prefix, descend through each slot continuation to the origin, write fallbacks and placeholders, unwind suffixes
    - id: commit
      action: flush the initial bytes; status and headers become final
    - id: fan-out
      action: range every async member sequence concurrently
    - id: merge
      action: serialize each resolved data:async-boundary-content through one coordinator in completion order
    - id: chunk
      action: write the identified template and update record, then flush that chunk
    - id: finish
      action: end when all member sequences are exhausted, the consumer stops, or the request context cancels
  failure:
    before_commit: yield zero content with the error and emit no chunk
    member_error: flow:suspense-html-render recover path inside the failing member only
    sibling_isolation: surviving members keep streaming
    cancellation: stop all member sequences without further chunks
```
