---
id: api:render-html-chain
type: api
title: Render HTML Chain
---
Compose an ordered list of bound components into one document and stream it, without filesystem route discovery.

```yaml
source:
  - requirement:chain-render-pipeline
  - user composition request 2026-07-25
owner: decision:generated-render-plan coordinator in the shared HTML runtime
relation: manual-handler counterpart of api:register-generated-html-routes
conceptual_signature:
  sync: func Render(w io.Writer, chain ...Component) error
  async: func RenderAsync(ctx context.Context, w io.Writer, chain ...Component) iter.Seq2[Content, error]
  collecting: func CollectChain(w io.Writer, key []byte, wrappers []Wrapper, leaf Fragment) (Manifest, error)
update_boundaries:
  members: each chain member declaring a boundary becomes one data:component-update-manifest instance, which is how requirement:layout-reuse-boundaries activates before filesystem routing exists
  excluded: the decision:html-document-shell member, because partial navigation retains the shell
  identity: chain position, so a member keeps its instance ID when only its parameters change
  frame: a member's validator covers its own markup and excludes the output of nested boundaries
  key: the caller supplies the validator key, so rule:update-validator-computation keying stays outside the render path and works without crypto/rand on constrained targets
  opt_in: only the collecting entry emits instance attributes, so the ordinary entries keep byte-identical output per requirement:html-rendering-compatibility
member:
  type: the decision:async-component-signature bound component value
  built_by: a generated binder pairing a component with its params struct
  carries: plan, params, requirement:head-merging contributions, and async capability
  lifetime: immutable and reusable, so a caller may keep members or a whole slice across requests
  no_chain_type: the chain is an argument list, not a reified value; a reusable chain is an ordinary slice
order:
  outermost_first: index zero is the outermost wrapper, typically decision:html-document-shell
  nesting: each member fills the next member into its unnamed requirement:html-slot-syntax slot
  innermost: the last member is the origin page and fills no one
named_slots:
  filled_by: each member own params, not by the chain array
  reason: only the unnamed slot expresses wrapping, so the array stays a single-axis list
execution:
  classification: requirement:chain-render-pipeline treats the chain as async when any member is
  phases: head merge, then initial pass, then merged boundary completions
  selection: a chain with no async member uses the sync entry and returns a plain error
validation:
  timing: before any byte is written, so a failure can still change the response status
  rules:
    - an empty chain is an error
    - every member except the last declares an unnamed slot
    - the last member must not have a required unnamed slot
    - a member appearing twice is an error
  note: static call sites keep their generation-time checks; these rules cover the runtime-assembled chain
flushing:
  problem: io.Writer has no flush, yet requirement:suspense-html-streaming depends on early bytes reaching the client
  rule: the coordinator flushes when the supplied writer implements a Flush method, checked by interface assertion rather than reflection
  points: after the initial pass and after each emitted data:async-boundary-content
  fallback: a writer without Flush still produces correct output, only without progressive delivery
acceptance:
  - a handler renders document plus page by passing two bound values
  - inserting a layout between them changes only the argument list
  - a chain whose page owns an await boundary streams fallbacks first and completions after
  - a chain missing a slot fails before the response is committed
open_questions:
  - helper for the common document plus page pair versus always spelling out the chain
  - whether the sync entry is a separate function or the async entry with an empty sequence
```
