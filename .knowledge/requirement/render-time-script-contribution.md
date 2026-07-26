---
id: requirement:render-time-script-contribution
type: requirement
title: Render Time Script Contribution
---
Let a render call add head script for that response alone, because a generation-time whitelist cannot express a script needed only on some paths.

```yaml
priority: should
source:
  - requirement:framework-script-contribution
  - user render-time decision 2026-07-27
review_gate: proposed
problem:
  too_early: generation-time registration fixes one contribution set for every response the generation unit produces
  path_specific: a script needed only for certain routes or certain handlers cannot be expressed by a whitelist that has no notion of a request
  cost: forcing such a script to be always-included makes every other page pay for it
precedent:
  existing: api:render-html-chain already does exactly this, prepending the boundary update runtime on the async entries while the sync entries add nothing
  observation: that runtime is needed only by components that actually open an await boundary, which is a render-time fact about the assembled chain
  generalization: this requirement turns that hardcoded special case into one channel any framework can use
channel:
  form: functional option on the render entries, beside the existing cache, context, error_hook, timeout, and concurrency members
  entries: Render, RenderAsync, RenderChain, RenderChainAsync
  timing: supplied at the call, so the set is complete before the head pass and before the first body byte
forms:
  named:
    shape: include a requirement:framework-script-contribution registration by name for this render
    works_with_inline: the generator already extracted and hashed its file, so an inline-source registration is includable this way with no per-request file writing
    validation: an unknown name fails before any byte is written
    preferred: true
  ad_hoc:
    shape: an external URL plus its decision:script-load-mode-authoring load mode
    use_for: a URL not known at generation time
    constraint: no inline source; requirement:static-asset-extraction writes files at generation time only
head_merging_amendment:
  existing_rule: requirement:head-merging requires contributions to be statically known markup, not conditional on request data
  amendment: a contribution may also arrive as a render-call argument
  preserved_invariant: every contribution is still known before the root head is written, so no body byte is buffered and streaming is unaffected
  still_forbidden:
    - a contribution discovered while walking a plan
    - a contribution produced by a requirement:render-value-provider
    - any contribution added after the root head is written
  distinction: the original rule bans contributions that wait for a render result; a call argument is available strictly earlier than that
ordering:
  group: render-supplied contributions merge after generation-time contributions and before requirement:static-asset-extraction component scripts
  reason: they may depend on the always-included base, and component script may depend on them
  within_group: the order supplied at the call
  detail: decision:framework-script-delivery
validation:
  timing: before the first byte, so a failure can still change the response status
  rules:
    - unknown contribution name
    - missing load mode on an ad hoc entry
    - a namespace already installed by a generation-time contribution or by another render-supplied entry
  boundary: a generation-time namespace collision stays a generation error; only the render-supplied case is necessarily checked at render time
dedup:
  identity: resolved reference URL, so a render-supplied entry duplicating a generation-time one collapses to one tag
  idempotent: supplying one contribution twice in one call emits one tag
caching:
  response: a complete response whose head varies per render cannot be shared by renders that selected a different set
  component: requirement:component-output-cache is unaffected, because head contributions never live in a cached component body
acceptance:
  - a handler adds a script for one route without the generation unit registering it as always-included
  - a page not selecting it emits no tag for it
  - the async boundary runtime is expressible through this channel instead of a hardcoded entry-point behavior
  - a render-supplied name never registered fails before the response is committed
  - the same contribution supplied twice emits one tag
  - an inline-source registration is includable by name at render time with no per-request file writing
  - the root head is still written before any body byte
related:
  - requirement:html-runtime-bootstrap
  - requirement:chain-render-pipeline
  - concept:framework-template-extensions
open_questions:
  - whether data:html-render-route-plan carries a default contribution set, so filesystem routes get per-route scripts without handler code
  - whether an ad hoc entry may declare a namespace at all, given no generation-time check can cover it
  - whether requirement:html-runtime-bootstrap is refactored onto this channel or stays a separate capability-selected tag
```
