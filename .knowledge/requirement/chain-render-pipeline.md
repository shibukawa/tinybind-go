---
id: requirement:chain-render-pipeline
type: requirement
title: Chain Render Pipeline
---
Classify and execute a whole layout-to-origin chain as one unit, because a slot owner cannot know its child before execution.

```yaml
source:
  - requirement:nested-layout-composition
  - user pipeline decision 2026-07-25
problem: composition slot capability accepts an unknown html continuation, so async classification cannot be a property of the slot owner alone
chain_assembly:
  origin: page or exported entry component that fills the innermost slot
  members: origin plus its ordered layout ancestors, outermost first
  plan: data:html-render-route-plan for generated routes
  timing:
    generated_plan: chain is fixed at generation time and its classification is precomputed
    runtime_override: chain is assembled per request and classified then
classification:
  rule: the chain is async when any member is async_boundary in data:component-render-capabilities
  sync_chain: use the existing requirement:template-code-generation writer path unchanged
  async_chain: expose the merged decision:async-component-signature sequence for the whole chain
  slot_owner: compiled async-agnostic; a layout never fixes the classification of whatever fills its slot
  no_partial_mode: a sync member inside an async chain still runs through the async driver
initial_pass:
  order: sequential and nested; outermost frame prefix inward through each slot continuation to the origin, then unwinding suffixes
  target: the single io.Writer argument
  content: static markup plus each decision:async-boundary-syntax fallback and its placeholder
  laziness: a member runs only when its parent actually renders its requirement:html-slot-syntax slot
  dropped_member: a member below an unrendered conditional slot starts no work, opens no boundary, and is excluded from the completion pass
  end: flush; the response status and headers are committed
completion_pass:
  concurrency: range every async member sequence concurrently
  merge: one coordinator serializes writes; emission follows completion order, not chain order
  emit: each data:async-boundary-content becomes an identified template plus update record, flushed as its own chunk
  transport: chunked transfer encoding when the protocol and negotiated encoding allow; requirement:suspense-html-streaming keeps the fallback for other transports
  end: all member sequences are exhausted, the consumer stops, or the request context cancels
identity:
  - boundary IDs come from one per-request namespace shared by the whole chain
  - merged updates from different members can never collide
  - a layout boundary and an origin boundary are indistinguishable to the browser runtime
failure:
  before_commit: the merged sequence yields zero content with the error before any chunk is written
  member_error: normalize as data:async-render-error and route to that member's own recover clause
  after_commit: a failed member cannot abort sibling members; the rest of the chain keeps streaming
  cancellation: stop every member sequence and emit no further chunks
safety:
  - only the ranging caller writes; member goroutines never touch the writer
  - a member sequence is single-use and single-consumer
  - concurrent members share no mutable render state
  - rule:template-context-safety applies independently inside each member
acceptance:
  - a sync layout wrapping an async page produces a valid async chain without regenerating the layout
  - an async layout wrapping a sync page streams its own boundaries the same way
  - a slow boundary in one member does not delay a resolved boundary in another
  - a chain with no async member emits byte-identical output to the current writer path
open_questions:
  - whether the merged sequence is generated per route or assembled by a shared runtime driver
  - concurrency limit across members versus per-member boundary limits
  - interaction with requirement:component-delta-rendering when a navigation delta and chain streaming overlap
```
