---
id: requirement:live-boundary-liveness-signal
type: requirement
title: Per-Boundary Liveness Signal
---
Say which boundary is live where the answer is needed: on the placeholder a client parses, and on the delivery a server routes, so neither side has to infer liveness from a second delivery that has not arrived yet.

```yaml
priority: should
source:
  - downstream framework live integration report 2026-07-31
  - requirement:live-boundary-rendering
  - concept:live-boundary-updates
review_gate: proposed
status: not implemented; liveness is reported per chain and nowhere per boundary
problem:
  identical_placeholder: the await and live ops write byte-identical markup, `<tb-boundary id="tb-N" style="display:contents">`, so the document carries no classification a parser could read
  identical_record: data:async-boundary-content is the BoundaryID and HTML pair for both, so a live-mode response interleaves settled await completions with live deliveries and a consumer cannot separate them
  wrong_granularity: requirement:fragment-capability-introspection HasLiveBlock answers per chain, which is the right unit for deciding whether to load a client runtime and the wrong one for addressing one region
occurrence:
  every_document: a page holding any live boundary, because the client must hold a durable address for every boundary it might later be asked to update
  every_reconnect: a requirement:live-boundary-resume response carries settle-once deliveries beside live ones with nothing to select on
why_not_inferable:
  finding: liveness cannot be recovered downstream at any price, which is what separates this from an ergonomics request
  node_reference:
    idea: remember the region by the DOM nodes it rendered, so no marker is needed
    breaks_on: an empty render; a list that yields nothing leaves no node behind and the next delivery has no place to go
    rule: an address must be independent of the content it addresses
  learn_on_second_delivery:
    idea: classify a boundary as live the first time it delivers twice
    breaks_on: timing; the address is already needed at the moment that delivery arrives, so the answer is always one delivery late
  keep_the_element:
    idea: leave `<tb-boundary>` in the applied DOM instead of replacing it
    breaks_on: it participates in CSS selectors and layout, which a comment does not, so it changes the page it was only meant to mark
downstream_cost:
  state: worked around and paid; the residual cost cannot be removed from outside
  dom: two comment nodes per applied boundary, permanent, on every boundary rather than on the live ones
  client_code: 43 of 341 lines for bracketing, refill, and stale-range handling; three lines if only settle-once boundaries had to be handled
  wire: every settled boundary's HTML retransferred on every reconnect, at a default lifetime of one rotation per client per ten minutes
proposal:
  two_answers:
    placeholder_attribute: a marker on the placeholder element, readable while the client parses the document, answering whether this boundary will be re-rendered
    content_field: a field on the delivery record, answering whether this delivery came from a live source, so a server can drop or route it without a side table
  both_wanted: they answer different questions at different times, and either alone leaves the other unanswered
  gain: the permanent per-boundary cost is confined to boundaries that are actually live instead of applying to every boundary of every page
  additive: an added attribute and an added struct field; requirement:html-rendering-compatibility holds because a template with no live binding emits neither
forward_compatibility:
  manifest: requirement:live-boundary-rendering already plans a data:component-update-manifest entry marking an instance live, so this is the same fact in the shipped id-and-html form rather than a competing one
  migration: when requirement:component-delta-rendering lands, the manifest entry supersedes the record field and the placeholder attribute stays, because a parser still reads the document before any manifest is applied
acceptance:
  - a client classifies every boundary while parsing the document, with no delivery required
  - a consumer of the live-mode sequence separates await completions from live deliveries with no side table
  - a page with no live binding emits the same bytes as before
  - a live boundary whose first render is empty is still addressable
open_questions:
  - whether the placeholder marker is an attribute on the boundary element or a distinct element name, given a distinct name changes what a caller's CSS and client code match on
  - whether the record field also distinguishes a failure delivery, which rule:live-boundary-delivery gives a revision like any other
```
