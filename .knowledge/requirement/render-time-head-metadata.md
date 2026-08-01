---
id: requirement:render-time-head-metadata
type: requirement
title: Render Time Head Metadata
---
Let a render call carry head metadata for that response alone, so a title derived from loaded data needs no second fetch and no separate declaration.

```yaml
priority: should
source:
  - decision:route-feature-ownership
  - requirement:render-time-script-contribution
  - user metadata decision 2026-07-27
review_gate: proposed
status: shipped 2026-07-31 as WithHead over typed nodes; the singleton conflict rules below are not implemented, because the runtime merge deduplicates identical tags and enforces no per-kind uniqueness
as_built:
  nodes: HeadTitle, HeadMeta, HeadLink, HeadScript, and HeadNoScript, each carrying attribute pairs rather than markup
  never_inline: HeadScript requires a src, so no path through this package writes inline script
  validation: a malformed node fails the render before the first byte, through an error the entry returns
  position: after every component contribution, deduplicated against them by exact tag
  fragment_path: RenderHeadNodes turns the same nodes into tags without a render, so a caller answering a request with no shell can see its own contributions rather than have them go nowhere
problem:
  derived_values: a page title and description usually come from the same record the page body renders
  separate_declaration: a template metadata block or a separate metadata method has no access to that record, so it must fetch again
  static_block: even with access, a generation-time block cannot hold a per-request value
  conclusion: the metadata belongs at the render call, where the loaded data is already in scope
generalization:
  existing: requirement:render-time-script-contribution already carries script contributions through a render option
  this: the same channel widened from script nodes to the rest of the head
  shared_invariant: the contribution arrives before the head pass, so requirement:head-merging still writes the root head before any body byte
channel:
  form: functional option on the render entries of api:render-html-chain, beside cache, context, timeout, concurrency, and scripts
  entries: Render, RenderAsync, RenderChain, RenderChainAsync
  timing: supplied at the call, so the set is complete before the head pass
  optional: a render call passing nothing behaves exactly as today
pending_values:
  limit: a value still pending through requirement:awaitable-parameters cannot appear in the head, because the root head is written before the first body byte
  consequence: a title derived from slow work forces the handler to await that work before rendering, which trades streaming for a correct title
  guidance: load what the head needs synchronously and leave the streaming work for the body
  not_a_defect: this is the same ordering requirement:head-merging already imposes, made visible rather than introduced
downstream_confirmation:
  when: 2026-07-31, per decision:library-component-seams
  reached_independently: the reporter asked for a declared way in for the caller rather than for a component, stating the same reason and the same limit, that a call argument is available strictly before the head pass so streaming is unaffected
  their_driver: a capability added after a project was scaffolded reaches no existing project, because the shell is a file the author owns; this is the upgrade-time delivery channel
  their_condition: not injection into the byte stream, but data the merge already knows how to order and deduplicate, in a stated position relative to component contributions
  blocking_gap: the node-kind set below, because the feature they are holding needs a noscript element and requirement:head-merging accepts none
content:
  allowed: title, meta, link, and the other node kinds requirement:head-merging already accepts
  noscript:
    needed_by: a scriptless-client handoff, which is one noscript refresh emitted only when a cookie is absent, so it is a caller decision by construction
    why_only_here: a static template cannot ask whether the cookie is already set, and an unconditional refresh in a scaffolded head refreshes forever
    status: open in requirement:head-merging; this channel is where it would be reachable at all
  escaping: values are untrusted data escaped for their head context, per rule:template-context-safety
  typed: the option carries typed nodes rather than a markup string, so a value cannot inject an element
merge:
  precedence: a render-call contribution is the innermost contributor, so its title wins over a component-declared default
  dedup: identical nodes still collapse, per requirement:head-merging
  singleton: charset and viewport conflicts stay generation or render errors rather than emitting two values
  scripts: requirement:render-time-script-contribution contributions continue to travel on their own option; this one adds no script handling
layering:
  htmlbind: owns the option and the merge, because it owns requirement:head-merging
  framework: decides how an author declares metadata and how it reaches this call, per decision:route-feature-ownership
  tinybind_generation: adds nothing; the option is a runtime API, not a generated shape
constraints:
  - no reflection; the option carries typed values
  - htmlbind gains no net/http dependency, so decision:runtime-package-boundaries is unchanged
  - a contribution cannot be added after the root head is written
acceptance:
  - a handler that loaded a record sets the document title from it with one fetch and one render call
  - a render call passing no metadata produces the same output as before this requirement
  - a title supplied at the render call overrides a component-declared default
  - a value containing markup characters is escaped rather than parsed
  - a handler that finds a cookie absent contributes a marker for that response alone, and the next request with the cookie set contributes nothing
open_questions:
  - option and node type names
  - whether an open-graph or structured-data helper ships, or only the generic node forms
  - whether a delta rerender may change head metadata, given that the head is already written
```
