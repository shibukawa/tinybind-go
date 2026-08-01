---
id: decision:live-boundary-syntax
type: decision
title: Live Sources In The Await Clause
---
Bind a live source in the ordinary await clause, because how often a value arrives is what its declaration says rather than what the wait site asks for.

```yaml
source:
  - concept:live-boundary-updates
  - decision:async-boundary-syntax
  - user design discussion 2026-07-30
review_gate: proposed
status: shipped, including several bindings in one clause and clauses mixing the two source kinds.
shape: |
  {await point = WatchMetrics(id)}
    primary subtree, re-rendered per delivery
  {fallback}
    subtree emitted before the first delivery
  {recover err}
    subtree replacing the boundary on terminal failure
  {/await}
semantics:
  clause: decision:async-boundary-syntax unchanged; a binding may name a decision:live-external-signature source as well as a settle-once one
  rebinding: each delivery re-binds the names and re-renders the primary subtree; the boundary output is a pure function of the current bindings plus reconstructed inputs
  fallback: emitted and committed before any delivery arrives, exactly as for a settle-once boundary
  recover: replaces the boundary on terminal failure and ends the subscription; optional, with the decision:async-boundary-syntax omitted_recover rule unchanged
  error: typed data:async-render-error, so a live source reports failures in the vocabulary the recover clause already reads
no_second_keyword:
  decided: no live clause keyword and no {/live} terminator, approved 2026-07-31
  reason: the await clause never constrained a boundary to one change; it says which values the subtree waits for, and the declaration says how often each arrives, so a keyword at the wait site would only repeat what `external live` already said
  superseded: a `{live}` clause with its own terminator, which shipped first and was removed the same day
  what_it_bought: only the ability to forbid mixing the two source kinds in one clause, which turned out to be a composition worth having rather than a case worth rejecting
  mixed_clause:
    legal: one clause may bind a settle-once source and a live one together
    behaviour: the settle-once binding delivers once and satisfies the wait; the live one keeps delivering, and every render reads both
    example: a panel whose title is fetched once and whose gauge keeps moving
    coalescing: values a live binding produces while a settle-once binding is still pending are coalesced, because the boundary cannot render until every binding has a value and the newest one is sufficient by construction
  readability_cost:
    is: a reader of the clause cannot tell from the wait site alone whether the boundary re-renders
    accepted: the declaration is in the same module, and it is the same thing the clause already does not say about how slow a binding is
  migration_property: a source changing from settle-once to live changes no wait site, which is the decision:async-component-signature argument that a component gains or loses a boundary without changing any call site
snapshot_binding:
  decided: a delivery carries the complete input for the boundary, not an increment
  means: a source watching a chat channel yields the current message list; a source sampling a gauge yields the current window
  reason:
    - the render stays side-effect-free and safe to repeat, which requirement:partial-update-boundaries already requires of every update boundary
    - a resume needs only the next delivery, with no replay and no server-held accumulation per subscription
    - a missed or coalesced delivery is not a lost delivery, which is what lets rule:live-boundary-delivery drop intermediates
  wire_cost: bounded by requirement:component-delta-rendering, which compares descendant validators and sends insert operations for an appended list rather than the whole list
  render_cost: the primary subtree is re-rendered whole per delivery, so a long list costs its length each time; that is the accepted cost of the snapshot model
  accumulation: whoever owns the source holds the accumulated list, because the template holds no state between deliveries
  deferred: an append-only clause that binds one item and emits one operation, which would trade the pure-function property for lower render cost
scoping:
  primary: outer scope plus the bound names, re-entered per delivery
  fallback: outer scope only; no delivery has arrived
  recover: outer scope plus the error name, and never the bound names
  no_carry: the primary subtree cannot read the previous delivery; a source that needs history yields it
concurrency:
  start: bindings of one boundary start together, each in its own goroutine
  first_render: the boundary waits until every binding has a current value, because the primary subtree reads them all and would otherwise show a zero one
  later_renders: any binding moving re-renders the whole subtree from the latest value of every binding
  no_selector:
    decided: a delivery carries every current value, so nothing has to say which source moved, approved 2026-07-31
    and_therefore: mixing source kinds needs no rule of its own, because a settle-once binding is one that delivers once
    reason: putting all the bindings on every render is what removes the need for a discriminated delivery type and a selector in the template; the alternative was restricting a clause to one binding precisely to avoid that question
    scope_shape: unchanged from an await clause, which already gives each binding its own scope field; only who writes those fields, and when, is different
  failure: the first failure decides the boundary whether or not the other bindings have arrived, since there is nothing left to wait for once recover is going to render
  ordering: deliveries are serialized, so two bindings moving at once cannot put an older render on screen after a newer one
compiler:
  - liveness is derived from the bindings rather than parsed, so it is one lookup at emission and no field on the clause
  - a live source may be called only in an await binding, which is the rule an async external already follows
  - every decision:async-boundary-syntax rule applies unchanged, including no slot inside the clause, one boundary per loop iteration, and nested clauses opening their own boundary
  - a boundary with at least one live binding is a live boundary for rule:live-boundary-content, so the control diagnostic follows the boundary rather than the binding
  - a live boundary is a requirement:partial-update-boundaries boundary implicitly, so it needs no separate update flag
capability: decision:component-capability-lowering raises a live flag beside the await flag, so requirement:fragment-capability-introspection can report it and a caller can decide whether to include a live client script at all
composition_with_await:
  legal: a live boundary's primary subtree may contain a nested boundary, which re-opens per delivery
  cost: that re-runs the awaited work on every delivery, so a note in the diagnostics is warranted rather than a prohibition
sync_entry:
  behavior: the decision:async-component-signature sync entry renders a live boundary from its first delivery and then stops watching, so one template serves a static request and a live one
  reason: the same argument the sync entry already makes for await boundaries, which is that a template must render correctly without progressive delivery
  audience: this is the path a crawler, a feed reader, and a browser with no JavaScript take
first_delivery_deadline:
  decided: on the entries that must produce a response, a boundary that has shown nothing stops waiting at the boundary deadline, approved 2026-07-31
  applies_to: the sync entry and the document entry; never the live entry, where a quiet source is normal rather than late
  outcome:
    sync: renders the fallback, which is the only honest answer when no value arrived
    document: leaves the committed fallback and unsubscribes
  not_a_failure:
    rule: running out of time here does not render recover and reports nothing to the error hook
    reason: a fallback is honest about a value that has not arrived, where a recover subtree would claim one went wrong; a live source being quiet is not a fault
    contrast: an await binding's timeout stays a failure, because the page needs the one value it was promised
  mixed_clause: a settle-once binding in a live boundary is bounded the same way, so a live boundary's deadline always means nothing to show yet rather than something failed
  one_delivery_is_enough: after the first render these entries stop watching, so the source's own yield reports false and the subscription closes; the deadline is only ever live while the boundary still has nothing to show
  problem_it_fixed: without it a source that never delivered held the response open until the request context ended, because the boundary deadline was deliberately not applied to a live subscription
open_questions:
  - whether the clause takes a declared minimum interval, or pacing stays entirely a property of the source
  - whether an initial-value form lets the boundary render primary content immediately and skip the fallback commit
  - whether the deferred append-only clause is a second keyword or a modifier on this one
```
