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
  - user options request 2026-07-26
owner: decision:generated-render-plan coordinator in the shared HTML runtime
relation: manual-handler counterpart of api:register-generated-html-routes
conceptual_signature:
  sync: func RenderChain(w io.Writer, wrappers []Wrapper, leaf Fragment, options ...Option) error
  async: func RenderChainAsync(ctx context.Context, w io.Writer, wrappers []Wrapper, leaf Fragment, options ...Option) iter.Seq2[Content, error]
  single: Render and RenderAsync are the wrapper-free forms of the same two
  helper: an exported Flush(w) so the ranging caller can push each written chunk without owning the writer-capability check
  naming: the progressive entry is named for asynchrony rather than streaming, which would collide with chunked transfer encoding and server-sent events
  no_convenience_wrapper: decision:async-component-signature keeps one async entry; see one_async_entry
member:
  type: the decision:async-component-signature bound component value
  built_by: a generated binder pairing a component with its params struct
  carries: plan, params, and requirement:head-merging contributions
  lifetime: immutable and reusable, so a caller may keep members or a whole slice across requests
  no_chain_type: the chain is an argument list, not a reified value; a reusable chain is an ordinary slice
options:
  form: variadic functional options, so existing two-argument call sites keep compiling
  reason: per-request resources belong to the caller, and a package-level default would be shared by every server in the process
  members:
    cache: api:cache-store used by decision:cache-component-declaration components
    context: request context for the sync entry, where no ctx parameter exists
    error_hook: receives the original Go error behind every boundary failure, including ones a recover subtree rendered; called from each boundary goroutine, so an accumulating hook guards its own state
    timeout: per-boundary deadline applied to requirement:async-external-functions work
    concurrency: upper bound on simultaneously running boundary work
    scripts: proposed requirement:render-time-script-contribution head script selected for this response alone
order:
  outermost_first: index zero is the outermost wrapper, typically decision:html-document-shell
  nesting: each member fills the next member into its unnamed requirement:html-slot-syntax slot
  innermost: the last member is the origin page and fills no one
named_slots:
  filled_by: each member own params, not by the chain array
  reason: only the unnamed slot expresses wrapping, so the array stays a single-axis list
execution:
  classification: requirement:chain-render-pipeline treats the chain as async when any member opens a boundary
  phases: head merge, then initial pass, then merged boundary completions
  selection: the caller chooses the entry; a chain with no boundary work yields nothing from the async entry
validation:
  timing: before any byte is written, so a failure can still change the response status
  rules:
    - an empty chain is an error
    - every member except the last declares an unnamed slot
    - the last member must not have a required unnamed slot
  note: static call sites keep their generation-time checks; these rules cover the runtime-assembled chain
flushing:
  problem: io.Writer has no flush, yet requirement:suspense-html-streaming depends on early bytes reaching the client
  rule: the coordinator flushes when the supplied writer implements a Flush method, checked by interface assertion rather than reflection
  points: the runtime flushes after the initial pass; the caller calls the exported helper after writing each data:async-boundary-content
  fallback: a writer without Flush still produces correct output, only without progressive delivery
bootstrap:
  rule: no entry injects script; per decision:client-runtime-ownership the caller supplies the boundary update runtime after asking requirement:fragment-capability-introspection, and the merged head carries component contributions only
  history: the async entries prepended a fixed update runtime until 2026-07-27
  driver: the caller's script defines the commit marker element in requirement:suspense-html-streaming and applies each boundary from its connected callback, never by watching for templates
  omission: the sync entries never needed one either, because a settled document needs no client runtime
  generalization: proposed requirement:render-time-script-contribution is the channel that carries such a script, replacing the removed entry-point prepend
acceptance:
  - a handler renders document plus page by passing two bound values
  - inserting a layout between them changes only the argument list
  - a chain whose page owns an await boundary streams fallbacks first and completions after
  - a chain missing a slot fails before the response is committed
  - passing a store makes a cached member reuse output across requests, and omitting it changes only performance
open_questions:
  - helper for the common document plus page pair versus always spelling out the chain
```
