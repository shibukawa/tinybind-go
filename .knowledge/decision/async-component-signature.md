---
id: decision:async-component-signature
type: decision
title: Async Component Render Signature
---
Give only components owning an await boundary an iterator-returning render signature; keep the synchronous writer signature everywhere else.

```yaml
source:
  - requirement:template-code-generation
  - requirement:html-component-api
  - user signature decision 2026-07-25
review_gate: approved 2026-07-25
baseline: requirement:html-component-api fragment signature and generated params struct
executor: decision:generated-render-plan coordinator; the entry function is a typed facade over it
sync_api: func Component(w io.Writer, params ComponentParams) error
async_api: func Component(ctx context.Context, w io.Writer, params ComponentParams) iter.Seq2[Content, error]
slot_type:
  shape: the decision:generated-render-plan component value, not a bare render func
  carries: bound plan, bound params, requirement:head-merging contributions, and async capability
  reason: a bare func hides head contributions, which the coordinator needs before writing the root head
content: data:async-boundary-content
context_argument:
  placement: leading parameter, not a generated params field
  reason: the params struct mirrors declared template parameters, and a synthesized ctx field would collide with the naming rule
selection:
  async_api: data:component-render-capabilities async_effect is async_boundary, or the assembled chain is async in requirement:chain-render-pipeline
  sync_api: every other capability set, preserving requirement:html-rendering-compatibility
  slot_owner: compiled async-agnostic; the chain classification decides which entry the caller uses
rationale:
  - static markup outside every await block still streams straight to the writer, so slot filling completes normally
  - only await block interiors leave through the sequence, so slot composition and async never compete for the same channel
  - only later boundary completions need a value channel, so only they need the iterator
  - iter.Seq2 lets the caller pull, flush, and stop without exposing goroutines or channels
  - error is the sequence error value, so both before-commit and after-commit failures use one path
  - a pending-only component never becomes exported; rule:component-capability-combinations forces an owning await boundary first
execution:
  start: rendering begins on the first pull; the initial pass writes fallback markup and placeholders to w
  yield: one data:async-boundary-content per completed boundary in completion order
  error: yield zero Content with the error; the sequence ends
  stop: early consumer stop cancels remaining request-owned work through ctx
  end: sequence ends when all request-owned boundaries settle or ctx cancels
caller:
  route_handler: requirement:generated-html-route-handlers ranges one merged sequence, wraps each item, and flushes
  chain: requirement:chain-render-pipeline ranges every async member sequence concurrently and merges them
  nested_call: application code never ranges a member sequence directly
constraints:
  - goroutines never touch w; only the ranging caller writes
  - the sequence is single-use and single-consumer
  - a component with no boundary work still writes its document and yields nothing
open_questions:
  - Content package placement and whether it is shared with requirement:component-delta-rendering operations
  - push-style iter.Seq2 versus an explicit pull adapter for callers mixing both writers
```
