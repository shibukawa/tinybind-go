---
id: decision:async-component-signature
type: decision
title: Async Render Entry Signature
---
Keep one uniform bound-component value and put the sync-versus-async split on the render entry point, not on each generated component function.

```yaml
source:
  - requirement:template-code-generation
  - requirement:html-component-api
  - user signature decision 2026-07-25
  - decision:generated-render-plan consequence 2026-07-26
review_gate: approved 2026-07-25; entry-point placement approved 2026-07-26
baseline: requirement:html-component-api fragment signature and generated params struct
executor: decision:generated-render-plan coordinator; the entry function is a typed facade over it
component_value:
  shape: one bound plan plus params value for every component, async or not
  carries: bound plan, bound params, and requirement:head-merging contributions
  reason: the coordinator walks plans, so awaiting is a runtime step rather than a property of the generated function type
  consequence: a component gains or loses an await boundary without changing the type of any call site that composes it
superseded:
  earlier: two generated component signatures, one returning error and one returning iter.Seq2
  why: the plan coordinator already erased per-component control flow, so a second signature only duplicated the classification in generated code
entries:
  sync: func Render(w io.Writer, leaf, options...) error
  async: func RenderAsync(ctx context.Context, w io.Writer, leaf, options...) iter.Seq2[Content, error]
  chain: api:render-html-chain carries the wrapper list variant of each
  naming: the progressive entry is named for its asynchrony, not for streaming, because a stream name collides with chunked transfer encoding and server-sent events
one_async_entry:
  decision: no error-returning wrapper hides the range loop, approved 2026-07-26
  reason: the number of boundaries a render produces is not knowable up front, least of all for a chain assembled at request time, so a streaming handler is written against the sequence either way
  consequence: the caller writes each Content and calls the exported flush helper, which is the same loop the wrapper would have contained
content: data:async-boundary-content
context_argument:
  placement: leading parameter of the async entries, not a generated params field
  reason: the params struct mirrors declared template parameters, and a synthesized ctx field would collide with the naming rule
  sync_entry: a render option may still supply a context, because api:cache-store and blocking await both want one
sync_of_async:
  behavior: the sync entry renders an await boundary by blocking on its bindings and emitting the settled primary or recover subtree in place
  reason: one template then renders correctly with or without progressive delivery, which also serves clients without JavaScript
  cost: no fallback is streamed and total latency is the slowest binding
selection:
  by_caller: the caller picks the entry; nothing in generated code forces the async entry
  chain: requirement:chain-render-pipeline classification decides only whether the async entry has work to yield
execution:
  start: rendering begins on the first pull; the initial pass writes fallback markup and placeholders to w and flushes
  yield: one data:async-boundary-content per settled boundary in completion order
  error: yield zero Content with the error; the sequence ends
  stop: early consumer stop cancels remaining request-owned work through ctx
  end: sequence ends when all request-owned boundaries settle or ctx cancels
caller:
  route_handler: requirement:generated-route-registration ranges the merged sequence, writes each item, and flushes
  loop: the caller ranges, writes each item, and flushes; nothing in the runtime does it on the caller's behalf
constraints:
  - goroutines never touch w; only the ranging caller and the initial pass write
  - the sequence is single-use and single-consumer
  - a render with no boundary work writes its document and yields nothing
open_questions:
  - Content package placement and whether it is shared with requirement:component-delta-rendering operations
```
